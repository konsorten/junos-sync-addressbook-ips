package main

import (
	"reflect"
	"testing"
)

func TestDiffSortedIPs4(t *testing.T) {
	a := []string{"aa", "ab/32", "ba", "bb", "ca/32"}
	b := []string{"ab", "ac", "ba", "bc/32", "cb"}
	x := []string{"aa/32", "ac/32", "bb/32", "bc/32", "ca/32", "cb/32"}

	c := DiffSortedIPs(a, b)

	if !reflect.DeepEqual(x, c) {
		t.Fatalf("Result is unexpected: %+v", c)
	}
}

func TestDiffSortedIPs6(t *testing.T) {
	a := []string{"a:a", "a:b/128", "b:a", "b:b", "c:a/128"}
	b := []string{"a:b", "a:c", "b:a", "b:c/128", "c:b"}
	x := []string{"a:a/128", "a:c/128", "b:b/128", "b:c/128", "c:a/128", "c:b/128"}

	c := DiffSortedIPs(a, b)

	if !reflect.DeepEqual(x, c) {
		t.Fatalf("Result is unexpected: %+v", c)
	}
}
