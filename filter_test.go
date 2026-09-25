package main

import (
	"reflect"
	"testing"
)

func TestIsIgnored(t *testing.T) {
	patterns := []string{"node_modules", ".git", "*.log"}

	cases := map[string]bool{
		"node_modules": true,
		".git":         true,
		"debug.log":    true,
		"main.go":      false,
		"src":          false,
	}

	for name, want := range cases {
		if got := isIgnored(name, patterns); got != want {
			t.Errorf("isIgnored(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestSplitPatterns(t *testing.T) {
	got := splitPatterns(" foo , bar,,")
	if want := []string{"foo", "bar"}; !reflect.DeepEqual(got, want) {
		t.Errorf("splitPatterns = %v, want %v", got, want)
	}
}
