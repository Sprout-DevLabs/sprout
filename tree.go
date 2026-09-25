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
	MaxDepth   int // -1 means unlimited
	Ignore     []string
}

// Node is one file or directory in the tree.
type Node struct {
	Name     string
	Path     string
	IsDir    bool
	Size     int64
	Children []*Node
	Err      error // set when a directory couldn't be read; the walk continues
}

// BuildTree walks root according to opts and returns the resulting tree.
// It's the single traversal both PrintTree and PrintStats build on.
func BuildTree(root string, opts Options) (*Node, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}

	node := &Node{Name: filepath.Base(root), Path: root, IsDir: info.IsDir()}
	if node.IsDir {
		if err := populate(node, opts, 0); err != nil {
			return nil, err
		}
	} else {
		node.Size = info.Size()
	}

	return node, nil
}

func populate(node *Node, opts Options, depth int) error {
	if opts.MaxDepth >= 0 && depth >= opts.MaxDepth {
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

		if !opts.ShowHidden && len(name) > 0 && name[0] == '.' {
			continue
		}
		if isIgnored(name, opts.Ignore) {
			continue
		}

		childPath := filepath.Join(node.Path, name)
		info, err := entry.Info()
		if err != nil {
			// Skip entries we can't stat (e.g. broken symlinks) instead of
			// failing the whole walk.
			continue
		}

		child := &Node{
			Name:  name,
			Path:  childPath,
			IsDir: entry.IsDir(),
		}

		if child.IsDir {
			if err := populate(child, opts, depth+1); err != nil {
				return err
			}
		} else {
			child.Size = info.Size()
		}

		node.Children = append(node.Children, child)
	}

	return nil
}

// PrintTree renders node's children using the familiar ├──/└── connectors.
func PrintTree(w io.Writer, node *Node, prefix string) {
	for i, child := range node.Children {
		connector := "├── "
		nextPrefix := prefix + "│   "
		if i == len(node.Children)-1 {
			connector = "└── "
			nextPrefix = prefix + "    "
		}

		suffix := ""
		if child.IsDir {
			suffix = "/"
		}

		if child.Err != nil {
			suffix += "  [" + errReason(child.Err) + "]"
		}

		fmt.Fprintln(w, prefix+connector+child.Name+suffix)

		if child.IsDir {
			PrintTree(w, child, nextPrefix)
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
