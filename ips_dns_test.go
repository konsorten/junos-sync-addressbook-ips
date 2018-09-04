package main

import (
	"testing"
)

func TestGetIPsFromDNS(t *testing.T) {
	ips, err := getIPsFromDNS("dns://www.heise-netze.de")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) <= 0 {
		t.Fatalf("No records returned")
	}

	t.Logf("%+v", ips)
}
