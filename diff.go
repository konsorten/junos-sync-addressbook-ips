package main

import (
	"strings"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func DiffSortedIPs(a, b []string) []string {
	var i, j int
	aCount := len(a)
	bCount := len(b)
	r := make([]string, 0, max(aCount, bCount))

	for i < aCount && j < bCount {
		aa := strings.TrimSuffix(a[i], "/32")
		bb := strings.TrimSuffix(b[j], "/32")
		c := strings.Compare(aa, bb)

		switch c {
		case 0:
			// ignore, just move on
			i++
			j++
			break
		case -1:
			// aa is smaller
			r = append(r, aa)
			i++
			break
		case 1:
			// bb is smaller
			r = append(r, bb)
			j++
			break
		}
	}

	// append remaining tails
	for ; i < aCount; i++ {
		r = append(r, strings.TrimSuffix(a[i], "/32"))
	}

	for ; j < bCount; j++ {
		r = append(r, strings.TrimSuffix(b[j], "/32"))
	}

	return r
}
