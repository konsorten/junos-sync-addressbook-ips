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

func (e PingdomEntry) JunosName() string {
	if strings.HasPrefix(e.Guid, "pingdom-") {
		return e.Guid
	}

	return "pingdom-" + strings.TrimSuffix(e.Hostname, ".pingdom.com")
}

type PingdomRSS struct {
	XMLName       xml.Name       `xml:"rss"`
	Title         string         `xml:"channel>title"`
	LastBuildDate string         `xml:"channel>lastBuildDate"`
	Entries       []PingdomEntry `xml:"channel>item"`
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
	addressMapKeys := make([]string, 0, len(addressbook.AddressEntries))
	addressMap := make(map[string]*junos.AddressEntry)
	for i, a := range addressbook.AddressEntries {
		if strings.HasPrefix(a.Name, "pingdom-") {
			addressMap[a.IP] = &addressbook.AddressEntries[i]
			addressMapKeys = append(addressMapKeys, a.IP)
		}
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
	ipMapKeys := make([]string, 0, len(pingdom.Entries))
	ipMap := make(map[string]*PingdomEntry)
	for i, a := range pingdom.Entries {
		if a.IPv4 != "" && a.State == "Active" {
			ipMap[a.IPv4] = &pingdom.Entries[i]
			ipMapKeys = append(ipMapKeys, a.IPv4)
		}
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

		if addressEntry == nil {
			// add new entry
			commands = append(commands, fmt.Sprintf("set address \"%v\" %v", ipEntry.JunosName(), ipEntry.IPv4))
		} else {
			// remove existing entry
			commands = append(commands, fmt.Sprintf("delete address \"%v\"", addressEntry.Name))
		}
	}

	// build address-set
	const PingdomProbeServersAddressSetName = "pingdom-probe-servers"

	commands = append(commands, fmt.Sprintf("delete address-set \"%v\"", PingdomProbeServersAddressSetName))

	for _, key := range ipMapKeys {
		ipEntry := ipMap[key]

		commands = append(commands, fmt.Sprintf("set address-set \"%v\" address \"%v\"", PingdomProbeServersAddressSetName, ipEntry.JunosName()))
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
