package main

import (
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// gitRepo runs git scoped to the directory sprout was pointed at.
type gitRepo struct {
	dir    string
	prefix string // root's path inside the repo, e.g. "cmd/api/"; "" at the top
}

func openRepo(dir string) (*gitRepo, error) {
	r := &gitRepo{dir: dir}
	prefix, err := r.run("rev-parse", "--show-prefix")
	if err != nil {
		return nil, fmt.Errorf("%s is not inside a git repository", dir)
	}
	r.prefix = strings.TrimSpace(prefix)
	return r, nil
}

func (r *gitRepo) run(args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", r.dir}, args...)...).Output()
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return "", fmt.Errorf("git %s: %s", args[0], strings.TrimSpace(string(ee.Stderr)))
	}
	return string(out), err
}

// rel maps a repo-relative path from git output to one relative to root.
func (r *gitRepo) rel(repoPath string) (string, bool) {
	return strings.CutPrefix(repoPath, r.prefix)
}

func (r *gitRepo) branch() string {
	b, err := r.run("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(b)
}

// status returns a one-letter code per changed path under root:
// M modified, A added, D deleted, R renamed, ? untracked, U conflicted.
func (r *gitRepo) status() (map[string]string, error) {
	out, err := r.run("status", "--porcelain=v1", "-z", "--untracked-files=all", "--", ".")
	if err != nil {
		return nil, err
	}
	changes := map[string]string{}
	fields := strings.Split(out, "\x00")
	for i := 0; i < len(fields); i++ {
		e := fields[i]
		if len(e) < 4 {
			continue
		}
		xy, p := e[:2], e[3:]
		if xy[0] == 'R' || xy[0] == 'C' {
			i++ // -z puts the rename source in the next field
		}
		code := string(xy[1])
		switch {
		case xy == "??":
			code = "?"
		case xy[0] == 'U' || xy[1] == 'U' || xy == "AA" || xy == "DD":
			code = "U"
		case xy[1] == ' ':
			code = string(xy[0])
		}
		if rel, ok := r.rel(p); ok {
			changes[rel] = code
		}
	}
	return changes, nil
}

// applyChanges marks each changed path on the tree, inserts paths that no
// longer exist on disk (deleted files) so they show in place, and counts
// changes on every ancestor directory.
func applyChanges(t *Tree, changes map[string]string) {
	counts := map[string]int{}
	for p, code := range changes {
		if n := insertPath(t, p); n != nil {
			n.Status = code
		}
		for d := path.Dir(p); d != "."; d = path.Dir(d) {
			counts[d]++
		}
	}
	walk(t.Root, func(n *Node) {
		if n.IsDir {
			n.Changes = counts[n.Rel]
		}
	})
}

// insertPath finds the node for rel, creating it (and any missing parent
// directories) if it isn't on disk. It returns nil when rel falls inside a
// directory the walk didn't expand (depth limit) or one it filtered out.
func insertPath(t *Tree, rel string) *Node {
	cur := t.Root
	parts := strings.Split(rel, "/")
	for i, name := range parts {
		if cur.Truncated || t.hides(name) {
			return nil
		}
		idx := sort.Search(len(cur.Children), func(j int) bool { return cur.Children[j].Name >= name })
		if idx < len(cur.Children) && cur.Children[idx].Name == name {
			cur = cur.Children[idx]
			continue
		}
		child := &Node{Name: name, Rel: strings.Join(parts[:i+1], "/"), IsDir: i < len(parts)-1, Missing: true}
		cur.Children = append(cur.Children, nil)
		copy(cur.Children[idx+1:], cur.Children[idx:])
		cur.Children[idx] = child
		cur = child
	}
	return cur
}

func walk(n *Node, fn func(*Node)) {
	fn(n)
	for _, c := range n.Children {
		walk(c, fn)
	}
}
