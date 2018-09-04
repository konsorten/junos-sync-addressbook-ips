package main

import (
	"testing"
)

func TestGetIPsFromURL(t *testing.T) {
	ips, err := getIPsFromURL("https://www.cloudflare.com/ips-v6")
	if err != nil {
		t.Fatal(err)
	}
	if len(ips) <= 0 {
		t.Fatalf("No records returned")
	}

	t.Logf("%+v", ips)
}
