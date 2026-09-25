package main

import (
	"fmt"
	"path"
	"strings"
)

// churn counts, for every file and directory under root, how many commits
// touched it. since, if set, is any date git understands ("90 days ago",
// "2026-01-01").
//
// ponytail: reads the whole log in one go; fine for most repos, pass
// --since on very large histories.
func (r *gitRepo) churn(since string) (map[string]int, error) {
	args := []string{"log", "--format=%x1e", "--name-only", "--relative", "--no-renames", "-z"}
	if since != "" {
		args = append(args, "--since="+since) // one argument, so it can't smuggle options
	}
	out, err := r.run(append(args, "--", ".")...)
	if err != nil {
		return nil, err
	}

	counts := map[string]int{}
	for _, commit := range strings.Split(out, "\x1e") {
		seen := map[string]bool{}
		for _, p := range strings.Split(commit, "\x00") {
			// A path and every directory above it count once per commit.
			for p = strings.Trim(p, "\n"); p != "" && p != "." && !seen[p]; p = path.Dir(p) {
				seen[p] = true
				counts[p]++
			}
		}
	}
	return counts, nil
}

func applyChurn(root *Node, counts map[string]int) {
	walk(root, func(n *Node) { n.Churn = counts[n.Rel] })
}

// churnBar renders n as one of eight block heights relative to max.
func churnBar(n, max int) string {
	const bars = "▁▂▃▄▅▆▇█"
	if n <= 0 || max <= 0 {
		return ""
	}
	i := (n*8 - 1) / max
	return string([]rune(bars)[i]) + fmt.Sprintf(" %d", n)
}

// churnMax returns the busiest file and directory counts, so files and
// directories are each scaled against their own kind.
func churnMax(root *Node) (files, dirs int) {
	walk(root, func(n *Node) {
		if n == root {
			return
		}
		if n.IsDir {
			dirs = max(dirs, n.Churn)
		} else {
			files = max(files, n.Churn)
		}
	})
	return files, dirs
}
