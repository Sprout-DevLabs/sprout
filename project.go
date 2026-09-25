package main

import (
	"os"
	"path/filepath"
)

// ProjectInfo is one ecosystem detected from a well-known manifest file.
type ProjectInfo struct {
	Manifest       string `json:"manifest"`
	Language       string `json:"language"`
	PackageManager string `json:"packageManager"`
}

var projectMarkers = []ProjectInfo{
	{"go.mod", "Go", "go modules"},
	{"package.json", "JavaScript/TypeScript", "npm"},
	{"pyproject.toml", "Python", "pip/poetry/uv"},
	{"requirements.txt", "Python", "pip"},
	{"Cargo.toml", "Rust", "cargo"},
	{"pom.xml", "Java", "maven"},
	{"build.gradle", "Java/Kotlin", "gradle"},
	{"build.gradle.kts", "Kotlin", "gradle"},
	{"Gemfile", "Ruby", "bundler"},
	{"composer.json", "PHP", "composer"},
	{"Package.swift", "Swift", "swiftpm"},
	{"mix.exs", "Elixir", "mix"},
	{"deno.json", "TypeScript", "deno"},
}

// DetectProject returns every ecosystem whose manifest sits in root, so a
// Go backend with a package.json frontend reports both.
func DetectProject(root string) []ProjectInfo {
	var found []ProjectInfo
	for _, m := range projectMarkers {
		if _, err := os.Stat(filepath.Join(root, m.Manifest)); err == nil {
			found = append(found, m)
		}
	}
	return found
}
