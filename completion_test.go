package main

import (
	"strings"
	"testing"
)

func TestCompletionsCoverEveryFlag(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
		out, _, code := runCLI(t, "--completion", shell)
		if code != 0 {
			t.Fatalf("%s: exit %d", shell, code)
		}
		for _, f := range allFlags() {
			want := dash(f.name)
			if shell == "fish" {
				want = "-l " + f.name
				if len(f.name) == 1 {
					want = "-s " + f.name
				}
			}
			if !strings.Contains(out, want) {
				t.Errorf("%s completion is missing %s", shell, want)
			}
		}
	}
	if _, errOut, code := runCLI(t, "--completion", "tcsh"); code != 2 || !strings.Contains(errOut, "bash, zsh") {
		t.Errorf("unknown shell: exit %d %s", code, errOut)
	}
}

func TestManPage(t *testing.T) {
	out, _, code := runCLI(t, "--man")
	if code != 0 || !strings.HasPrefix(out, ".TH SPROUT 1") {
		t.Fatalf("exit %d: %.80s", code, out)
	}
	for _, f := range allFlags() {
		if !strings.Contains(out, ".B "+strings.ReplaceAll(dash(f.name), "-", `\-`)) {
			t.Errorf("man page is missing %s", dash(f.name))
		}
	}
}
