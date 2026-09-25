package main

import (
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

// defaultIgnores are common development artifacts hidden outside git repos,
// where there's no .gitignore to tell us what matters.
var defaultIgnores = []string{
	"node_modules",
	"dist",
	"build",
	".cache",
	"__pycache__",
	".venv",
	"venv",
	".idea",
	".vscode",
	"target",
	"obj",
	"coverage",
	".next",
	".nuxt",
	".pytest_cache",
	".mypy_cache",
	".DS_Store",
	"*.egg-info",
}

// splitPatterns parses the comma-separated --ignore value.
func splitPatterns(s string) []string {
	var patterns []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// isIgnored checks a bare file/dir name against glob-style patterns.
func isIgnored(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
	}
	return false
}

// gitVisible asks git which paths under root belong to the project: tracked
// files plus untracked files that aren't gitignored, and every parent
// directory of those. ok is false outside a work tree or without git.
//
// ponytail: an untracked nested repo is listed as "dir/" and shows up empty;
// walk into it separately if that ever matters.
func gitVisible(root string) (map[string]bool, bool) {
	out, err := exec.Command("git", "-C", root, "ls-files",
		"--cached", "--others", "--exclude-standard", "-z").Output()
	if err != nil {
		return nil, false
	}
	set := map[string]bool{}
	for _, p := range strings.Split(string(out), "\x00") {
		for p = strings.TrimSuffix(p, "/"); p != "" && p != "." && !set[p]; p = path.Dir(p) {
			set[p] = true
		}
	}
	return set, true
}
