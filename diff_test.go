package main

import (
	"reflect"
	"testing"
)

func TestDiffSortedIPs(t *testing.T) {
	a := []string{"aa", "ab/32", "ba", "bb", "ca/32"}
	b := []string{"ab", "ac", "ba", "bc/32", "cb"}
	x := []string{"aa", "ac", "bb", "bc", "ca", "cb"}

	c := DiffSortedIPs(a, b)

	if !reflect.DeepEqual(x, c) {
		t.Fatalf("Result is unexpected: %+v", c)
	}
}
