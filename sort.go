package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
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

// parseAge reads --changed-within values: Go durations (90m, 36h) plus
// d for days and w for weeks (7d, 2w).
func parseAge(s string) (time.Duration, error) {
	for suffix, unit := range map[string]time.Duration{"d": 24 * time.Hour, "w": 7 * 24 * time.Hour} {
		if n, ok := strings.CutSuffix(s, suffix); ok {
			v, err := strconv.ParseFloat(n, 64)
			if err != nil || v < 0 {
				break
			}
			return time.Duration(v * float64(unit)), nil
		}
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("--changed-within wants a duration like 30m, 12h, 7d or 2w, not %q", s)
	}
	return d, nil
}

// capFiles keeps the first max files in each directory (in the current
// sort order) and records how many were left out. Directories always stay.
func capFiles(root *Node, max int) {
	walk(root, func(d *Node) {
		kept, files := d.Children[:0], 0
		for _, c := range d.Children {
			if !c.IsDir {
				if files++; files > max {
					d.More++
					continue
				}
			}
			kept = append(kept, c)
		}
		d.Children = kept
	})
}
