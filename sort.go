package main

import (
	"fmt"
	"sort"
	"strings"
)

// sortTree orders every directory's children. by is "name" (the walk's
// default order), "size" (largest first) or "time" (newest first); reverse
// flips it, and dirsFirst lists directories before files.
func sortTree(n *Node, by string, reverse, dirsFirst bool) {
	less := func(a, b *Node) bool { return a.Name < b.Name }
	switch by {
	case "size":
		less = func(a, b *Node) bool { return a.Size > b.Size }
	case "time":
		less = func(a, b *Node) bool { return a.ModTime.After(b.ModTime) }
	}
	walk(n, func(d *Node) {
		sort.SliceStable(d.Children, func(i, j int) bool {
			a, b := d.Children[i], d.Children[j]
			if dirsFirst && a.IsDir != b.IsDir {
				return a.IsDir
			}
			if reverse {
				return less(b, a)
			}
			return less(a, b)
		})
	})
}

func validSort(by string) error {
	switch by {
	case "name", "size", "time":
		return nil
	}
	return fmt.Errorf("--sort must be name, size or time, not %q", by)
}

// humanSize is du -h style: 512B, 4.2K, 1.3M. si uses powers of 1000.
func humanSize(n int64, si bool) string {
	unit := int64(1024)
	if si {
		unit = 1000
	}
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := unit, 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	s := fmt.Sprintf("%.1f", float64(n)/float64(div))
	return strings.TrimSuffix(s, ".0") + string("KMGTPE"[exp])
}
