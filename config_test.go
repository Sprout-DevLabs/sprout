package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Keep a developer's own ~/.config/sprout/config out of every test.
func TestMain(m *testing.M) {
	os.Setenv("SPROUT_CONFIG", filepath.Join(os.TempDir(), "sprout-test-no-such-config"))
	os.Exit(m.Run())
}

func TestProjectConfig(t *testing.T) {
	dir := setupTestDir(t)
	write(t, dir, ".sproutrc", "# defaults for this repo\n--depth 1\n--ignore=README.md\n")

	out, _, code := runCLI(t, dir)
	if code != 0 || strings.Contains(out, "main.go") || strings.Contains(out, "README.md") {
		t.Errorf(".sproutrc not applied:\n%s", out)
	}

	// Command line wins over the file.
	out, _, _ = runCLI(t, dir, "--depth", "-1")
	if !strings.Contains(out, "main.go") {
		t.Errorf("--depth on the command line should override .sproutrc:\n%s", out)
	}

	// Found from a subdirectory too.
	out, _, _ = runCLI(t, filepath.Join(dir, "src"), "--depth=-1", "--ignore", "nothing")
	if !strings.Contains(out, "main.go") {
		t.Errorf("run from subdir:\n%s", out)
	}

	out, _, _ = runCLI(t, dir, "--no-config")
	if !strings.Contains(out, "README.md") {
		t.Errorf("--no-config should skip .sproutrc:\n%s", out)
	}
}

func TestUserConfigAndPatternsAccumulate(t *testing.T) {
	dir := setupTestDir(t)
	user := filepath.Join(t.TempDir(), "config")
	os.WriteFile(user, []byte("--ignore README.md\n"), 0o644)
	t.Setenv("SPROUT_CONFIG", user)

	out, _, _ := runCLI(t, dir, "--ignore", "src")
	if strings.Contains(out, "README.md") || strings.Contains(out, "src") {
		t.Errorf("user config and command-line --ignore should both apply:\n%s", out)
	}
}

func TestBadConfig(t *testing.T) {
	dir := setupTestDir(t)
	write(t, dir, ".sproutrc", "depth 3\n")
	_, errOut, code := runCLI(t, dir)
	if code != 2 || !strings.Contains(errOut, ".sproutrc:1") {
		t.Errorf("want exit 2 naming the file and line, got %d: %s", code, errOut)
	}

	write(t, dir, ".sproutrc", "--nope\n")
	_, errOut, code = runCLI(t, dir)
	if code != 2 || !strings.Contains(errOut, "config") {
		t.Errorf("unknown flag in config: exit %d: %s", code, errOut)
	}
}
