package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// sprout check: the imports a project forbids. Rules live in .sproutrules
// at the project root, one per line:
//
//	# the web app never reaches the database directly
//	web/ -> db/
//	**/*.tsx -> server/**
//
// Each side is a glob on paths from the root: * stays within a folder, **
// crosses folders, and a trailing slash means everything under the folder.
// A file matching the left that depends on a file matching the right breaks
// the rule. Tests are left out of the left side: they ship nowhere.

type checkRule struct {
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	from, to *regexp.Regexp
}

type violation struct {
	checkRule
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason,omitempty"`
}

type checkResult struct {
	SchemaVersion int         `json:"schemaVersion"`
	Command       string      `json:"command"`
	Checked       int         `json:"checked"` // files whose imports were checked
	Violations    []violation `json:"violations"`
	Unmatched     []checkRule `json:"unmatched,omitempty"` // rules a side of which matches no file: likely a typo
}

const rulesFile = ".sproutrules"

func runCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sprout check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {}
	diff := fs.String("diff", "", "only the files changed in a revision range, e.g. origin/main...HEAD")
	asJSON := fs.Bool("json", false, "print JSON")
	err := fs.Parse(args)
	if err == flag.ErrHelp {
		fmt.Fprint(stdout, "Usage: sprout check [--diff REV] [--json]\n\nReports imports that break the rules in "+rulesFile+" at the project root, one per line:\n\n  web/ -> db/          files under web/ can't depend on files under db/\n  **/*.tsx -> server/  no component imports server code\n\n--diff checks only the files changed in REV, so a rule can be adopted before old breaks are fixed.\nExits 1 when a rule is broken.\n")
		return 0
	}
	if err == nil && fs.NArg() > 0 {
		fmt.Fprintf(stderr, "sprout: check takes no arguments, got %q\n", fs.Arg(0))
		err = flag.ErrHelp
	}
	if err != nil {
		fmt.Fprintln(stderr, "Run 'sprout check --help' for usage.")
		return 2
	}
	root, err := impactRoot(nil)
	if err != nil {
		fmt.Fprintln(stderr, "sprout:", err)
		return 1
	}
	rules, err := readRules(filepath.Join(root, rulesFile))
	if os.IsNotExist(err) {
		fmt.Fprintf(stderr, "sprout: no %s in %s; add one with rules like: web/ -> db/\n", rulesFile, root)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "sprout:", err)
		return 2
	}
	g, err := projectGraph(root, false)
	if err != nil {
		fmt.Fprintln(stderr, "sprout:", err)
		return 1
	}
	var only map[string]bool // nil: every file
	if *diff != "" {
		_, changed, _, err := gitChanges(root, false, *diff, "")
		if err != nil {
			fmt.Fprintln(stderr, "sprout:", err)
			return 1
		}
		only = map[string]bool{}
		for _, c := range changed {
			only[c] = true
		}
	}
	res := checkGraph(g, rules, only)
	if *asJSON {
		data, _ := json.Marshal(res)
		fmt.Fprintf(stdout, "%s\n", data)
	} else {
		printCheck(stdout, res, len(rules), *diff)
	}
	if len(res.Violations) > 0 {
		return 1
	}
	return 0
}

// readRules parses a rules file: "FROM -> TO" per line, # for comments.
func readRules(p string) ([]checkRule, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rules []checkRule
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		from, to, ok := strings.Cut(line, "->")
		from, to = strings.TrimSpace(from), strings.TrimSpace(to)
		if !ok || from == "" || to == "" {
			return nil, fmt.Errorf("%s:%d: expected a rule like web/ -> db/, got %q", rulesFile, n, line)
		}
		rules = append(rules, checkRule{Line: n, Rule: from + " -> " + to, from: rulePattern(from), to: rulePattern(to)})
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("%s has no rules; add one like: web/ -> db/", rulesFile)
	}
	return rules, nil
}

// rulePattern compiles one side of a rule: a glob on the path from the
// root, with a trailing slash for everything under a folder.
func rulePattern(glob string) *regexp.Regexp {
	glob = strings.TrimPrefix(glob, "/")
	if strings.HasSuffix(glob, "/") {
		glob += "**"
	}
	return regexp.MustCompile("^" + globRegexp(glob) + "$") // globRegexp quotes everything it doesn't translate
}

// checkGraph finds every edge breaking a rule, from the non-test files in
// only (nil for all).
func checkGraph(g *Graph, rules []checkRule, only map[string]bool) checkResult {
	res := checkResult{SchemaVersion: 1, Command: "check", Violations: []violation{}}
	to := make([][]bool, len(rules)) // to[r][id]: file id matches rule r's right side
	for r, rule := range rules {
		var matched bool
		if to[r], matched = ruleTargets(g, rule); !matched {
			res.Unmatched = append(res.Unmatched, rule)
		}
	}
	checked := func(f GraphFile) bool { return !f.Test && (only == nil || only[f.Rel]) }
	for r, rule := range rules {
		for id, f := range g.Files {
			if !checked(f) || !rule.from.MatchString(f.Rel) {
				continue
			}
			for _, dep := range g.Deps(FileID(id)) {
				if to[r][dep] {
					res.Violations = append(res.Violations, violation{rule, f.Rel, g.Files[dep].Rel, g.Why(FileID(id), dep)})
				}
			}
		}
	}
	for _, f := range g.Files {
		if checked(f) {
			res.Checked++
		}
	}
	return res
}

// ruleTargets marks the files matching rule's right side; matched is
// whether both sides match a file.
func ruleTargets(g *Graph, rule checkRule) (to []bool, matched bool) {
	to = make([]bool, len(g.Files))
	from, target := false, false
	for id, f := range g.Files {
		to[id] = rule.to.MatchString(f.Rel)
		target = target || to[id]
		from = from || rule.from.MatchString(f.Rel)
	}
	return to, from && target
}

func printCheck(w io.Writer, res checkResult, rules int, diff string) {
	scope := plural(res.Checked, "file")
	if diff != "" {
		scope += " changed in " + diff
	}
	for _, u := range res.Unmatched {
		fmt.Fprintf(w, "%s:%d  %s matches no file on one side; check the paths\n", rulesFile, u.Line, printable(u.Rule))
	}
	if len(res.Violations) == 0 {
		fmt.Fprintf(w, "No imports break the %s in %s (%s checked)\n", plural(rules, "rule"), rulesFile, scope)
		return
	}
	verb := "break"
	if len(res.Violations) == 1 {
		verb = "breaks"
	}
	fmt.Fprintf(w, "%s %s %s (%s checked)\n", plural(len(res.Violations), "import"), verb, rulesFile, scope)
	line := 0
	for _, v := range res.Violations {
		if v.Line != line {
			line = v.Line
			fmt.Fprintf(w, "\n%s:%d  %s\n", rulesFile, v.Line, printable(v.Rule))
		}
		fmt.Fprintf(w, "  %s -> %s  %s\n", printable(v.From), printable(v.To), printable(v.Reason))
	}
}
