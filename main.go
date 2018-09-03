package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/scottdware/go-junos"
	log "github.com/sirupsen/logrus"
)

type GlobalAddressbook struct {
	XMLName        xml.Name             `xml:"configuration"`
	Name           string               `xml:"security>address-book>name"`
	AddressEntries []junos.AddressEntry `xml:"security>address-book>address"`
	AddressSets    []junos.AddressSet   `xml:"security>address-book>address-set"`
}

type PingdomEntry struct {
	XMLName     xml.Name `xml:"item"`
	Guid        string   `xml:"guid"`
	Title       string   `xml:"title"`
	Description string   `xml:"description"`
	IPv4        string   `xml:"ip"`
	IPv6        string   `xml:"ipv6"`
	Hostname    string   `xml:"hostname"`
	State       string   `xml:"state"`
	Country     string   `xml:"country"`
	City        string   `xml:"city"`
	Region      string   `xml:"region"`
}

func (e PingdomEntry) IsActiveIPv4() bool {
	return e.State == "Active" && e.IPv4 != ""
}

func (e PingdomEntry) IsActiveIPv6() bool {
	return e.State == "Active" && e.IPv6 != ""
}

func (e PingdomEntry) JunosName(ipv6 bool) string {
	var suffix string
	if ipv6 {
		suffix = "-v6"
	} else {
		suffix = "-v4"
	}

	if strings.HasPrefix(e.Guid, "pingdom-") {
		return e.Guid + suffix
	}

	return "pingdom-" + strings.TrimSuffix(e.Hostname, ".pingdom.com") + suffix
}

type PingdomRSS struct {
	XMLName       xml.Name       `xml:"rss"`
	Title         string         `xml:"channel>title"`
	LastBuildDate string         `xml:"channel>lastBuildDate"`
	Entries       []PingdomEntry `xml:"channel>item"`
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

	// rollback any previously existing changes
	jnpr.Rollback(nil)

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
	for i, a := range addressbook.AddressEntries {
		if strings.HasPrefix(a.Name, "pingdom-") {
			addressMap[unifyIP(a.IP)] = &addressbook.AddressEntries[i]
		}
	}

	addressMapKeys := make([]string, 0, len(addressMap))
	for ip := range addressMap {
		addressMapKeys = append(addressMapKeys, ip)
	}
	sort.Strings(addressMapKeys)

	// get IP list
	log.Infof("Downloading IP list...")

	ipXml, err := http.Get("https://my.pingdom.com/probes/feed")
	if err != nil {
		return err
	}

	ipXmlData, err := ioutil.ReadAll(ipXml.Body)
	if err != nil {
		return err
	}

	var pingdom PingdomRSS
	if err := xml.Unmarshal(ipXmlData, &pingdom); err != nil {
		return err
	}

	log.Debugf("  IP addresses found: %v", len(pingdom.Entries))

	// build IP map
	ipMap := make(map[string]*PingdomEntry)
	for i, a := range pingdom.Entries {
		if a.IsActiveIPv4() {
			ipMap[unifyIP(a.IPv4)] = &pingdom.Entries[i]
		}
		if a.IsActiveIPv6() {
			ipMap[unifyIP(a.IPv6)] = &pingdom.Entries[i]
		}
	}

	ipMapKeys := make([]string, 0, len(ipMap))
	for ip := range ipMap {
		ipMapKeys = append(ipMapKeys, ip)
	}
	sort.Strings(ipMapKeys)

	// compare entries
	updatedKeys := DiffSortedIPs(addressMapKeys, ipMapKeys)

	log.Infof("Updating %v entries...", len(updatedKeys))

	commands := make([]string, 0, 2*len(updatedKeys))
	commands = append(commands, "edit security address-book global")

	for _, key := range updatedKeys {
		addressEntry := addressMap[key]
		ipEntry := ipMap[key]
		isIPv6 := strings.Contains(key, ":")

		if addressEntry == nil {
			// add new entry
			if isIPv6 {
				commands = append(commands, fmt.Sprintf("set address \"%v\" %v", ipEntry.JunosName(true), ipEntry.IPv6))
			} else {
				commands = append(commands, fmt.Sprintf("set address \"%v\" %v", ipEntry.JunosName(false), ipEntry.IPv4))
			}
		} else {
			// remove existing entry
			commands = append(commands, fmt.Sprintf("delete address \"%v\"", addressEntry.Name))
		}
	}

	// build address-set
	const PingdomProbeServersAddressSetName = "pingdom-probe-servers"

	commands = append(commands, fmt.Sprintf("delete address-set \"%v\"", PingdomProbeServersAddressSetName))
	commands = append(commands, fmt.Sprintf("set address-set \"%v\" description \"Pingdom Probe Servers\"", PingdomProbeServersAddressSetName))

	for _, key := range ipMapKeys {
		ipEntry := ipMap[key]

		if ipEntry.IsActiveIPv4() {
			commands = append(commands, fmt.Sprintf("set address-set \"%v\" address \"%v\"", PingdomProbeServersAddressSetName, ipEntry.JunosName(false)))
		}
		if ipEntry.IsActiveIPv6() {
			commands = append(commands, fmt.Sprintf("set address-set \"%v\" address \"%v\"", PingdomProbeServersAddressSetName, ipEntry.JunosName(true)))
		}
	}

	commands = append(commands, "top")

	// apply changes
	log.Infof("Applying changes...")

	/*if log.IsLevelEnabled(log.DebugLevel) {
		for _, c := range commands {
			log.Debugf("  %v", c)
		}
	}*/

	defer jnpr.Rollback(nil)

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
