package main

import "testing"

func TestLanguageOf(t *testing.T) {
	cases := map[string]string{
		"main.go":    "Go",
		"App.TSX":    "TypeScript",
		"Makefile":   "Makefile",
		"Dockerfile": "Dockerfile",
		"data.toml":  "TOML",
		".env":       "",
		"LICENSE":    "",
	}
	for name, want := range cases {
		if got := languageOf(name); got != want {
			t.Errorf("languageOf(%q) = %q, want %q", name, got, want)
		}
	}
}
