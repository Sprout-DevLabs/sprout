package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}

	must(os.MkdirAll(filepath.Join(dir, "src"), 0755))
	must(os.MkdirAll(filepath.Join(dir, "node_modules"), 0755))
	must(os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0644))
	must(os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644))
	must(os.WriteFile(filepath.Join(dir, ".hidden"), []byte("secret"), 0644))

	return dir
}

func childNames(node *Node) map[string]bool {
	names := map[string]bool{}
	for _, c := range node.Children {
		names[c.Name] = true
	}
	return names
}

func TestBuildTreeBasic(t *testing.T) {
	dir := setupTestDir(t)

	root, err := BuildTree(dir, Options{MaxDepth: -1})
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	names := childNames(root)
	if !names["src"] || !names["README.md"] {
		t.Errorf("expected src and README.md in tree, got %v", names)
	}
	if names[".hidden"] {
		t.Error("hidden file should not appear by default")
	}
}

func TestBuildTreeShowHidden(t *testing.T) {
	dir := setupTestDir(t)

	root, err := BuildTree(dir, Options{ShowHidden: true, MaxDepth: -1})
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if !childNames(root)[".hidden"] {
		t.Error("expected .hidden to appear when ShowHidden is true")
	}
}

func TestBuildTreeDepthLimit(t *testing.T) {
	dir := setupTestDir(t)

	root, err := BuildTree(dir, Options{MaxDepth: 1})
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	for _, c := range root.Children {
		if c.Name == "src" && len(c.Children) != 0 {
			t.Error("expected src to have no children when MaxDepth is 1")
		}
	}
}

func TestBuildTreeIgnore(t *testing.T) {
	dir := setupTestDir(t)

	root, err := BuildTree(dir, Options{MaxDepth: -1, Ignore: []string{"node_modules"}})
	if err != nil {
		t.Fatalf("BuildTree failed: %v", err)
	}

	if childNames(root)["node_modules"] {
		t.Error("node_modules should have been ignored")
	}
}

func TestBuildTreeNonExistentPath(t *testing.T) {
	_, err := BuildTree(filepath.Join(t.TempDir(), "does-not-exist"), Options{})
	if err == nil {
		t.Error("expected an error for a non-existent path")
	}
}

func TestUnreadableDirDoesNotAbortWalk(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("chmod-based permission test needs a non-root Unix user")
	}
	dir := setupTestDir(t)
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	root, err := BuildTree(dir, Options{MaxDepth: -1})
	if err != nil {
		t.Fatalf("walk aborted: %v", err)
	}
	var found *Node
	for _, c := range root.Children {
		if c.Name == "locked" {
			found = c
		}
	}
	if found == nil || found.Err == nil {
		t.Error("expected locked/ in the tree with its read error recorded")
	}
	if !childNames(root)["src"] {
		t.Error("siblings of the unreadable dir should still be listed")
	}
}
