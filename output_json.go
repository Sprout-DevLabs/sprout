package main

import (
	"encoding/json"
	"io"
	"path/filepath"
)

// jsonSchemaVersion is bumped on any breaking change to the --json shape.
// Adding fields is not breaking.
const jsonSchemaVersion = 1

type jsonReport struct {
	SchemaVersion int           `json:"schemaVersion"`
	Name          string        `json:"name"`
	Root          string        `json:"root"`
	GitAware      bool          `json:"gitAware"`
	Projects      []ProjectInfo `json:"projects"`
	Summary       jsonSummary   `json:"summary"`
	Tree          *Node         `json:"tree"`
}

type jsonSummary struct {
	Directories int            `json:"directories"`
	Files       int            `json:"files"`
	Bytes       int64          `json:"bytes"`
	Skipped     int            `json:"skipped"`
	Languages   map[string]int `json:"languages"`
}

// MarshalJSON gives nodes an explicit "type" and a string error, which is
// what scripts and editor integrations want to switch on.
func (n *Node) MarshalJSON() ([]byte, error) {
	typ := "file"
	if n.IsDir {
		typ = "directory"
	}
	var errMsg string
	if n.Err != nil {
		errMsg = errReason(n.Err)
	}
	return json.Marshal(struct {
		Name      string  `json:"name"`
		Path      string  `json:"path,omitempty"`
		Type      string  `json:"type"`
		Size      int64   `json:"size,omitempty"`
		Error     string  `json:"error,omitempty"`
		Truncated bool    `json:"truncated,omitempty"`
		Missing   bool    `json:"missing,omitempty"`
		Status    string  `json:"status,omitempty"`
		Changes   int     `json:"changes,omitempty"`
		Children  []*Node `json:"children,omitempty"`
	}{n.Name, n.Rel, typ, n.Size, errMsg, n.Truncated, n.Missing, n.Status, n.Changes, n.Children})
}

func WriteJSON(w io.Writer, t *Tree, path string) error {
	s := &Stats{Languages: map[string]int{}}
	collectStats(t.Root, s)

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	projects := DetectProject(path)
	if projects == nil {
		projects = []ProjectInfo{} // [] not null, so consumers can iterate blindly
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(jsonReport{
		SchemaVersion: jsonSchemaVersion,
		Name:          filepath.Base(abs),
		Root:          filepath.ToSlash(abs),
		GitAware:      t.GitAware,
		Projects:      projects,
		Summary: jsonSummary{
			Directories: s.Directories,
			Files:       s.Files,
			Bytes:       s.TotalSize,
			Skipped:     t.Skipped,
			Languages:   s.Languages,
		},
		Tree: t.Root,
	})
}
