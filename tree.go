package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Options controls how the tree is built.
type Options struct {
	ShowHidden bool
	MaxDepth   int      // -1 means unlimited
	Ignore     []string // user patterns, always applied
	NoIgnore   bool     // disable .gitignore and the built-in ignore list
}

// Node is one file or directory in the tree.
type Node struct {
	Name     string
	Path     string // filesystem path, for reading
	Rel      string // slash-separated path relative to the root, for matching git output
	IsDir    bool
	Size     int64
	Children []*Node
	Err      error // set when a directory couldn't be read; the walk continues
}

// Tree is a walked directory plus what the walk left out.
type Tree struct {
	Root     *Node
	Skipped  int  // entries hidden by dotfile or ignore rules
	GitAware bool // .gitignore rules were applied
}

type walker struct {
	opts    Options
	ignore  []string
	visible map[string]bool // nil unless GitAware
	skipped int
}

// BuildTree walks root according to opts and returns the resulting tree.
// It's the single traversal every output mode builds on.
//
// Ignore rules: inside a git work tree, .gitignore is the source of truth
// (asked of git itself, so every gitignore feature is honoured). Outside git,
// a built-in list of common build/dependency directories is used instead.
func BuildTree(root string, opts Options) (*Tree, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	w := &walker{opts: opts, ignore: append([]string{".git"}, opts.Ignore...)}
	t := &Tree{}
	if !opts.NoIgnore {
		if w.visible, t.GitAware = gitVisible(root); !t.GitAware {
			w.ignore = append(w.ignore, defaultIgnores...)
		}
	}

	node := &Node{Name: displayName(root), Path: root, IsDir: info.IsDir()}
	if node.IsDir {
		if err := w.populate(node, 0); err != nil {
			return nil, err
		}
	} else {
		node.Size = info.Size()
	}

	t.Root, t.Skipped = node, w.skipped
	return t, nil
}

func (w *walker) populate(node *Node, depth int) error {
	if w.opts.MaxDepth >= 0 && depth >= w.opts.MaxDepth {
		return nil
	}

	// os.ReadDir already returns entries sorted by name.
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		if depth == 0 {
			return err
		}
		// One unreadable subdirectory (e.g. permission denied) shouldn't
		// abort the whole walk; record it and keep going.
		node.Err = err
		return nil
	}

	for _, entry := range entries {
		name := entry.Name()
		rel := name
		if node.Rel != "" {
			rel = node.Rel + "/" + name
		}

		if isIgnored(name, w.ignore) || (w.visible != nil && !w.visible[rel]) ||
			(!w.opts.ShowHidden && name[0] == '.') {
			w.skipped++
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue // vanished between ReadDir and Lstat
		}

		child := &Node{
			Name:  name,
			Path:  filepath.Join(node.Path, name),
			Rel:   rel,
			IsDir: entry.IsDir(),
		}

		if child.IsDir {
			if err := w.populate(child, depth+1); err != nil {
				return err
			}
		} else {
			child.Size = info.Size()
		}

		node.Children = append(node.Children, child)
	}

	return nil
}

// Count returns the number of directories and files below node.
func (n *Node) Count() (dirs, files int) {
	for _, c := range n.Children {
		if c.IsDir {
			d, f := c.Count()
			dirs, files = dirs+d+1, files+f
		} else {
			files++
		}
	}
	return dirs, files
}

// PrintTree renders node's children using the familiar ├──/└── connectors.
// annotate, if non-nil, returns extra text shown after an entry's name.
func PrintTree(w io.Writer, node *Node, prefix string, annotate func(*Node) string) {
	for i, child := range node.Children {
		connector := "├── "
		nextPrefix := prefix + "│   "
		if i == len(node.Children)-1 {
			connector = "└── "
			nextPrefix = prefix + "    "
		}

		line := prefix + connector + child.Name
		if child.IsDir {
			line += "/"
		}
		if child.Err != nil {
			line += "  [" + errReason(child.Err) + "]"
		}
		if annotate != nil {
			if a := annotate(child); a != "" {
				line += "  " + a
			}
		}

		fmt.Fprintln(w, line)

		if child.IsDir {
			PrintTree(w, child, nextPrefix, annotate)
		}
	}
}

// errReason strips the path from *PathError so the tree shows just
// "permission denied" next to the entry that already names the path.
func errReason(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
