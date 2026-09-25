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

	Truncated bool   // directory not expanded because of --depth
	Status    string // --git / --diff change code: M, A, D, R, ?, U
	Changes   int    // directories: changed paths anywhere below
	Added     int    // --diff: lines added (directories: total below)
	Deleted   int    // --diff: lines deleted (directories: total below)
	Churn     int    // --churn: commits that touched this path
}

// Tree is a walked directory plus what the walk left out.
type Tree struct {
	Root     *Node
	Skipped  int  // entries hidden by dotfile or ignore rules
	GitAware bool // .gitignore rules were applied

	hides func(name string) bool // the walk's name filter, reused for inserted paths
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

	t.Root, t.Skipped, t.hides = node, w.skipped, w.hides
	return t, nil
}

func (w *walker) hides(name string) bool {
	return isIgnored(name, w.ignore) || (!w.opts.ShowHidden && name[0] == '.')
}

func (w *walker) populate(node *Node, depth int) error {
	if w.opts.MaxDepth >= 0 && depth >= w.opts.MaxDepth {
		node.Truncated = true
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

		if w.hides(name) || (w.visible != nil && !w.visible[rel]) {
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

// printer renders a tree with the familiar ├──/└── connectors.
type printer struct {
	w     io.Writer
	color bool

	churnFiles, churnDirs int // --churn scale; 0 when off
}

func (p printer) tree(node *Node, prefix string) {
	for i, child := range node.Children {
		connector := "├── "
		nextPrefix := prefix + "│   "
		if i == len(node.Children)-1 {
			connector = "└── "
			nextPrefix = prefix + "    "
		}
		fmt.Fprintln(p.w, paint(p.color, dim, prefix+connector)+p.label(child))
		if child.IsDir {
			p.tree(child, nextPrefix)
		}
	}
}

func (p printer) label(n *Node) string {
	name := n.Name
	switch {
	case n.IsDir:
		name = paint(p.color, blue+";"+bold, name+"/")
	case n.Status == "D":
		name = paint(p.color, red, name)
	}
	if n.Status != "" {
		name += "  " + paint(p.color, statusColor[n.Status], n.Status)
	}
	if n.Changes > 0 {
		name += "  " + paint(p.color, dim, fmt.Sprintf("(%d changed)", n.Changes))
	}
	if n.Added+n.Deleted > 0 {
		name += "  " + paint(p.color, green, fmt.Sprintf("+%d", n.Added)) + " " + paint(p.color, red, fmt.Sprintf("-%d", n.Deleted))
	}
	if n.Churn > 0 {
		scale := p.churnFiles
		if n.IsDir {
			scale = p.churnDirs
		}
		c := dim
		if n.Churn*3 > scale*2 {
			c = red
		} else if n.Churn*3 > scale {
			c = yellow
		}
		name += "  " + paint(p.color, c, churnBar(n.Churn, scale))
	}
	if n.Err != nil {
		name += "  " + paint(p.color, red, "["+errReason(n.Err)+"]")
	}
	return name
}

// errReason strips the path from *PathError so the tree shows just
// "permission denied" next to the entry that already names the path.
func errReason(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
