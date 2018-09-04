package main

import (
	uurl "net/url"
	"time"

	"github.com/miekg/dns"
)

const (
	DnsServer string = "8.8.8.8:53"
)

func getIPsFromDNS(url string) ([]string, error) {
	u, err := uurl.Parse(url)
	if err != nil {
		return nil, err
	}

	hostname := u.Hostname() + "."
	ips := make([]string, 0)
	questions := []dns.Question{
		dns.Question{hostname, dns.TypeA, dns.ClassINET},
		dns.Question{hostname, dns.TypeAAAA, dns.ClassINET},
	}

	for _, q := range questions {
		m1 := new(dns.Msg)
		m1.Id = dns.Id()
		m1.RecursionDesired = true
		m1.Question = []dns.Question{q}

		c := new(dns.Client)
		c.Timeout = 15 * time.Second
		in, _, err := c.Exchange(m1, DnsServer)
		if err != nil {
			return nil, err
		}

		for _, answer := range in.Answer {
			switch record := answer.(type) {
			case *dns.A:
				ips = append(ips, unifyIP(record.A.String()))
				break
			case *dns.AAAA:
				ips = append(ips, unifyIP(record.AAAA.String()))
				break
			}
		}
	}

	return ips, nil
}
