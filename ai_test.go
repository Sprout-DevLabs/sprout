package main

import (
	"fmt"
	"strings"
	"testing"
)

func setupBigProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "README.md", "# demo\n\n[![ci](badge)](x)\n\nA tiny service that does demo things.\n")
	write(t, dir, "go.mod", "module demo\n")
	write(t, dir, "cmd/api/main.go", "package main")
	write(t, dir, "Dockerfile", "FROM scratch")
	write(t, dir, ".github/workflows/ci.yml", "on: push")
	write(t, dir, "docs/logo.gif", "x")
	for i := 0; i < 30; i++ {
		write(t, dir, fmt.Sprintf("internal/pkg%02d/file.go", i), "package x")
		write(t, dir, fmt.Sprintf("examples/ex%02d/main.go", i), "package main")
	}
	return dir
}

func TestAIMapHeader(t *testing.T) {
	out, errOut, code := runCLI(t, setupBigProject(t), "--ai")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{
		"A tiny service that does demo things.",
		"stack: Go (go.mod)",
		"entry points: cmd/api/main.go",
		"config: .github/workflows/ci.yml, Dockerfile",
		"## structure",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "GIF") {
		t.Errorf("binary assets shouldn't count as languages:\n%s", out)
	}
	if strings.Contains(out, "examples/ex00/main.go") {
		t.Error("examples/ shouldn't be listed as an entry point")
	}
}

func TestAIMapBudget(t *testing.T) {
	dir := setupBigProject(t)
	full, _, _ := runCLI(t, dir, "--ai", "--budget", "100000")
	small, _, _ := runCLI(t, dir, "--ai", "--budget", "250")

	if !strings.Contains(full, "pkg29/ file.go") || !strings.Contains(full, "ex29/ main.go") {
		t.Errorf("a large budget should render everything:\n%s", full)
	}
	if got := estimateTokens(small); got > 250*11/10 {
		t.Errorf("budget 250 produced ~%d tokens:\n%s", got, small)
	}
	// Project code is expanded before examples get any of the budget.
	if !strings.Contains(small, "examples/ (30 files, mostly .go)") {
		t.Errorf("examples/ should collapse first under a tight budget:\n%s", small)
	}
}
