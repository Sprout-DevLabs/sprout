package main

import (
	"encoding/json"
	"testing"
)

func TestJSONOutput(t *testing.T) {
	dir := setupTestDir(t)
	out, _, code := runCLI(t, dir, "--json")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}

	var r struct {
		SchemaVersion int
		Projects      []any
		Summary       struct{ Files, Directories, Skipped int }
		Tree          struct {
			Type     string
			Children []struct{ Name, Path, Type string }
		}
	}
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if r.SchemaVersion != 1 || r.Projects == nil {
		t.Errorf("schemaVersion=%d projects=%v", r.SchemaVersion, r.Projects)
	}
	if r.Summary.Files != 2 || r.Summary.Directories != 1 || r.Summary.Skipped != 2 {
		t.Errorf("summary = %+v", r.Summary)
	}
	if r.Tree.Type != "directory" {
		t.Errorf("root type = %q", r.Tree.Type)
	}
	for _, c := range r.Tree.Children {
		if c.Name == "src" && (c.Type != "directory" || c.Path != "src") {
			t.Errorf("src node = %+v", c)
		}
	}
}
