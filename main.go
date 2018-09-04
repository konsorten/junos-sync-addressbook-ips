package main

import (
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"hash/crc64"
	"os"
	"sort"
	"strings"

	"github.com/scottdware/go-junos"
	log "github.com/sirupsen/logrus"
	"github.com/tv42/zbase32"
)

type GlobalAddressbook struct {
	XMLName        xml.Name             `xml:"configuration"`
	Name           string               `xml:"security>address-book>name"`
	AddressEntries []junos.AddressEntry `xml:"security>address-book>address"`
	AddressSets    []junos.AddressSet   `xml:"security>address-book>address-set"`
}

type IPAddressEntry struct {
	IP        string
	IsIPv6    bool
	JunosName string
}

func unifyIP(ip string) string {
	if strings.Contains(ip, "/") {
		return ip
	}
	if strings.Contains(ip, ":") {
		return ip + "/128"
	}
	return ip + "/32"
}

func main() {
	err := mainInternal()
	if err != nil {
		log.Errorf("ERROR: %v", err)
		os.Exit(1)
	}
	log.Info("DONE.")
}

func mainInternal() error {
	log.SetLevel(log.DebugLevel)

	// gather data
	juniperHost := os.Getenv("JUNIPER_HOST")
	if juniperHost == "" {
		return fmt.Errorf("Missing environment variable: JUNIPER_HOST")
	}

	juniperUser := os.Getenv("JUNIPER_USER")
	if juniperUser == "" {
		return fmt.Errorf("Missing environment variable: JUNIPER_USER")
	}

	juniperPassword := os.Getenv("JUNIPER_PASSWORD")
	if juniperPassword == "" {
		return fmt.Errorf("Missing environment variable: JUNIPER_PASSWORD")
	}

	juniperAddressSetName := os.Getenv("JUNIPER_ADDRESS_SET")
	if juniperAddressSetName == "" {
		return fmt.Errorf("Missing environment variable: JUNIPER_ADDRESS_SET")
	}

	ipsSourceUrl := os.Getenv("IPS_SOURCE_URL")
	if ipsSourceUrl == "" {
		return fmt.Errorf("Missing environment variable: IPS_SOURCE_URL")
	}

	// perform login
	log.Infof("Connecting to %v...", juniperHost)

	auth := &junos.AuthMethod{
		Credentials: []string{juniperUser, juniperPassword},
	}

	jnpr, err := junos.NewSession(juniperHost, auth)
	if err != nil {
		return err
	}

	defer jnpr.Close()

	// lock the juniper config
	err = jnpr.Lock()
	if err != nil {
		return fmt.Errorf("Failed to lock Juniper config: %v", err)
	}

	defer jnpr.Unlock()

	// rollback any previously existing changes
	{
		diff, err := jnpr.Diff(0)
		if err != nil {
			return err
		}
		diffIsEmpty := strings.TrimSpace(diff) == ""

		if !diffIsEmpty {
			log.Infof("Rolling back uncommitted changes...")

			jnpr.Rollback(0)
		}
	}

	// retrieve the addressbook
	log.Infof("Reading global addressbook...")

	xmlData, err := jnpr.GetConfig("xml", "security>address-book")
	if err != nil {
		return err
	}

	var addressbook GlobalAddressbook
	if err := xml.Unmarshal([]byte(xmlData), &addressbook); err != nil {
		return err
	}

	log.Debugf("  Addresses found: %v", len(addressbook.AddressEntries))
	log.Debugf("  Address-Sets found: %v", len(addressbook.AddressSets))

	// build address map
	addressMap := make(map[string]*junos.AddressEntry)
	prefix := juniperAddressSetName + "-"
	for i, a := range addressbook.AddressEntries {
		if strings.HasPrefix(a.Name, prefix) {
			addressMap[unifyIP(a.IP)] = &addressbook.AddressEntries[i]
		}
	}

	addressMapKeys := make([]string, 0, len(addressMap))
	for ip := range addressMap {
		addressMapKeys = append(addressMapKeys, ip)
	}
	sort.Strings(addressMapKeys)

	// get IP list
	log.Infof("Retrieving IP lists...")

	ipAddressMap := make(map[string]*IPAddressEntry)
	{
		crc64Table := crc64.MakeTable(0x3405254d7ba559cd)
		urls := strings.Split(ipsSourceUrl, ";;")

		for _, url := range urls {
			log.Infof("  Downloading IPs from %v ...", url)

			var ips []string
			var err error

			if strings.HasPrefix(url, "http:") || strings.HasPrefix(url, "https:") {
				ips, err = getIPsFromURL(url)
			} else if strings.HasPrefix(url, "dns:") {
				ips, err = getIPsFromDNS(url)
			} else {
				log.Errorf("Unsupported URL schema: %v", url)
			}
			if err != nil {
				return err
			}

			for _, ip := range ips {
				crc := crc64.Checksum([]byte(ip), crc64Table)
				b := make([]byte, 8)
				binary.LittleEndian.PutUint64(b, crc)

				ipAddressMap[ip] = &IPAddressEntry{
					IP:        ip,
					IsIPv6:    strings.Contains(ip, ":"),
					JunosName: fmt.Sprintf("%v-%v", juniperAddressSetName, zbase32.EncodeToString(b)),
				}
			}
		}
	}

	ipAddresses := make([]string, 0, len(ipAddressMap))
	for ip := range ipAddressMap {
		ipAddresses = append(ipAddresses, ip)
	}
	sort.Strings(ipAddresses)

	log.Debugf("  IP addresses found: %v", len(ipAddresses))

	// compare entries
	updatedKeys := DiffSortedIPs(addressMapKeys, ipAddresses)

	log.Infof("Updating %v entries...", len(updatedKeys))

	commands := make([]string, 0, 2*len(updatedKeys))
	commands = append(commands, "edit security address-book global")

	for _, ip := range updatedKeys {
		addressEntry := addressMap[ip]
		ipEntry := ipAddressMap[ip]

		if addressEntry == nil {
			// add new entry
			commands = append(commands, fmt.Sprintf("set address \"%v\" %v", ipEntry.JunosName, ip))
		} else {
			// remove existing entry
			commands = append(commands, fmt.Sprintf("delete address \"%v\"", addressEntry.Name))
		}
	}

	// build address-set
	commands = append(commands, fmt.Sprintf("delete address-set \"%v\"", juniperAddressSetName))
	commands = append(commands, fmt.Sprintf("set address-set \"%v\" description \"IP addresses from %v\"", juniperAddressSetName, ipsSourceUrl))

	for _, ip := range ipAddresses {
		ipEntry := ipAddressMap[ip]

		commands = append(commands, fmt.Sprintf("set address-set \"%v\" address \"%v\"", juniperAddressSetName, ipEntry.JunosName))
	}

	commands = append(commands, "top")

	// apply changes
	log.Infof("Applying changes...")

	/*if log.IsLevelEnabled(log.DebugLevel) {
		for _, c := range commands {
			log.Debugf("  %v", c)
		}
	}*/

	defer func() {
		diff, err := jnpr.Diff(0)
		if err == nil {
			diffIsEmpty := strings.TrimSpace(diff) == ""

			if !diffIsEmpty {
				log.Infof("Rolling back uncommitted changes...")

				jnpr.Rollback(0)
			}
		}
	}()

	err = jnpr.Config(commands, "set", false)
	if err != nil {
		return err
	}

	diff, err := jnpr.Diff(0)
	if err != nil {
		return err
	}
	diffIsEmpty := strings.TrimSpace(diff) == ""
	if diffIsEmpty {
		log.Infof("No changes, nothing to commit.")
		return nil
	}

	if !diffIsEmpty && log.IsLevelEnabled(log.DebugLevel) {
		log.Debugf("----- DIFF\n%v\n----- DIFF", diff)
	}

	// commit
	log.Infof("Committing...")

	err = jnpr.Commit()
	if err != nil {
		if strings.Contains(err.Error(), "<ok>") {
			return nil
		}
		return err
	}

	return nil
}
