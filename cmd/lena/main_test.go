package main

import (
	"reflect"
	"testing"
)

func TestSplitAndTrim(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a, b, c", []string{"a", "b", "c"}},
		{"a,, b ", []string{"a", "b"}},
		{"", []string{}},
		{"  only  ", []string{"only"}},
	}

	for _, c := range cases {
		got := splitAndTrim(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("splitAndTrim(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
