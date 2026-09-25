package main

import (
	"os"
	"strings"
	"testing"
)

func TestGitStatus(t *testing.T) {
	dir := setupGitRepo(t)
	write(t, dir, "lib/gone.go", "x")
	write(t, dir, ".github/ci.yml", "x")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", "init")

	write(t, dir, "src/main.go", "package main // edited")
	write(t, dir, "new.txt", "x")
	write(t, dir, ".github/ci.yml", "edited")
	if err := os.Remove(dir + "/lib/gone.go"); err != nil {
		t.Fatal(err)
	}

	out, errOut, code := runCLI(t, dir, "--git")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"main.go  M", "new.txt  ?", "gone.go  D", "src/  (1 changed)", " on "} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, ".github") {
		t.Errorf("a change inside a hidden dir must not surface it:\n%s", out)
	}
}

func TestGitStatusFromSubdir(t *testing.T) {
	dir := setupGitRepo(t)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-qm", "init")
	write(t, dir, "src/main.go", "edited")

	out, _, code := runCLI(t, dir+"/src", "--git")
	if code != 0 || !strings.Contains(out, "main.go  M") {
		t.Errorf("repo-relative paths weren't mapped to the subdir root:\n%s", out)
	}
}

func TestGitOutsideRepo(t *testing.T) {
	if _, _, code := runCLI(t, t.TempDir(), "--git"); code != 1 {
		t.Errorf("expected exit 1 outside a repo, got %d", code)
	}
}
