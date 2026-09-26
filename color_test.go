package main

import (
	"runtime"
	"strings"
	"testing"
)

func TestHyperlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a Unix absolute path")
	}
	got := hyperlink("main.go", "/tmp/my proj/main.go", "box")
	want := "\x1b]8;;file://box/tmp/my%20proj/main.go\x1b\\main.go\x1b]8;;\x1b\\"
	if got != want {
		t.Errorf("hyperlink = %q, want %q", got, want)
	}
}

func TestHyperlinkNotInPipes(t *testing.T) {
	out, _, _ := runCLI(t, setupTestDir(t), "--hyperlink")
	if strings.Contains(out, "\x1b]8") {
		t.Errorf("escape codes leaked into non-terminal output: %q", out)
	}
}
