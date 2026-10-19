package main

import "testing"

func TestLanguageOf(t *testing.T) {
	cases := map[string]string{
		"main.go":      "Go",
		"App.TSX":      "TypeScript",
		"app.mjs":      "JavaScript",
		"app.cjs":      "JavaScript",
		"types.mts":    "TypeScript",
		"types.cts":    "TypeScript",
		"Main.kt":      "Kotlin",
		"build.kts":    "Kotlin",
		"App.swift":    "Swift",
		"Program.cs":   "C#",
		"config.toml":  "TOML",
		"App.vue":      "Vue",
		"App.svelte":   "Svelte",
		"main.dart":    "Dart",
		"main.ex":      "Elixir",
		"test.exs":     "Elixir",
		"Main.scala":   "Scala",
		"script.lua":   "Lua",
		"main.zig":     "Zig",
		"main.tf":      "Terraform",
		"schema.proto": "Protocol Buffers",
		"Makefile":     "Makefile",
		"Dockerfile":   "Dockerfile",
		".env":         "",
		"LICENSE":      "",
	}

	for name, want := range cases {
		if got := languageOf(name); got != want {
			t.Errorf("languageOf(%q) = %q, want %q", name, got, want)
		}
	}
}
