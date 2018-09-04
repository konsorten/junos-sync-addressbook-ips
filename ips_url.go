package main

import (
	"bufio"
	"net/http"
	"strings"
)

func getIPsFromURL(url string) ([]string, error) {
	ipXml, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(ipXml.Body)
	ips := make([]string, 0)

	for scanner.Scan() {
		ips = append(ips, unifyIP(strings.TrimSpace(scanner.Text())))
	}

	err = scanner.Err()
	if err != nil {
		return nil, err
	}

	return ips, nil
}
