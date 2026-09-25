package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

const usage = `sprout: map your codebase, for you and your AI agent

Usage:
  sprout [path] [flags]
  sprout mcp [root]       serve project_map, tree and diff_tree to coding agents (MCP, stdio)

Views:
  (default)               tree of the project, respecting .gitignore
  --ai                    compact project map for LLM prompts; --budget N tokens (default 2000)
  --diff REV              only what changed in REV, e.g. main...HEAD or HEAD~3
  --stats                 files, size, languages, detected stack
  --json                  tree and stats as JSON (schemaVersion 1)

Annotations:
  --git                   mark changed files: M modified, A added, D deleted, R renamed, ? untracked, U conflict
  --churn                 commits per path, to spot hotspots; --since '90 days ago'

Filtering:
  -L, --depth N           limit depth
  -a, --all               show hidden and ignored entries
  --hidden                show dotfiles
  --no-ignore             skip .gitignore and the built-in ignore list
  --ignore LIST           extra names/globs to hide, comma-separated: '*.log,fixtures'

  --version               print version

Examples:
  sprout -L 2
  sprout --ai | pbcopy
  sprout --diff main...HEAD -L 2
  sprout --churn --since '6 months ago'
`

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		os.Exit(serveMCP(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
	}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sprout", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {} // errors get a one-line hint below; --help gets usage on stdout

	var all bool
	fs.BoolVar(&all, "all", false, "show everything: hidden files and ignored entries")
	fs.BoolVar(&all, "a", false, "shorthand for --all")
	hidden := fs.Bool("hidden", false, "show hidden files and directories")
	noIgnore := fs.Bool("no-ignore", false, "don't apply .gitignore or the built-in ignore list")
	var depth int
	fs.IntVar(&depth, "depth", -1, "limit directory depth (-1 for unlimited)")
	fs.IntVar(&depth, "L", -1, "shorthand for --depth")
	ignore := fs.String("ignore", "", "comma-separated names/globs to ignore, e.g. '*.log,fixtures'")
	stats := fs.Bool("stats", false, "show project statistics instead of the tree")
	asJSON := fs.Bool("json", false, "print the tree and statistics as JSON")
	gitStatus := fs.Bool("git", false, "mark changed files with their git status")
	churn := fs.Bool("churn", false, "show how many commits touched each path (hotspots)")
	since := fs.String("since", "", "with --churn: only count commits since this date, e.g. '90 days ago'")
	aiMap := fs.Bool("ai", false, "print a compact project map for LLM prompts and agents")
	budget := fs.Int("budget", 2000, "with --ai: approximate token budget")
	diffRev := fs.String("diff", "", "show only paths changed in a git revision range, e.g. main...HEAD")
	showVersion := fs.Bool("version", false, "print version and exit")

	path, err := parseArgs(fs, args)
	if err == flag.ErrHelp {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if err != nil {
		fmt.Fprintln(stderr, "Run 'sprout --help' for usage.")
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, "sprout", resolveVersion())
		return 0
	}

	info, err := os.Stat(path)
	if err != nil {
		fmt.Fprintln(stderr, "sprout:", err)
		return 1
	}
	if !info.IsDir() {
		fmt.Fprintf(stderr, "sprout: %s is not a directory\n", path)
		return 1
	}

	opts := Options{
		// --ai wants .github/ and friends; ignore rules still drop the junk.
		ShowHidden: *hidden || all || *aiMap,
		MaxDepth:   depth,
		Ignore:     splitPatterns(*ignore),
		NoIgnore:   *noIgnore || all,
	}

	tree, branch, err := load(path, opts, *gitStatus, *diffRev)
	if err == nil && *churn {
		err = addChurn(path, tree, *since)
	}
	if err != nil {
		fmt.Fprintln(stderr, "sprout:", err)
		return 1
	}

	if *aiMap {
		fmt.Fprint(stdout, AIMap(tree, path, *budget))
		return 0
	}

	if *asJSON {
		if err := WriteJSON(stdout, tree, path); err != nil {
			fmt.Fprintln(stderr, "sprout:", err)
			return 1
		}
		return 0
	}

	if *stats {
		PrintStats(stdout, tree, path)
		return 0
	}

	p := printer{w: stdout, color: useColor(stdout)}
	if *churn {
		p.churnFiles, p.churnDirs = churnMax(tree.Root)
	}
	header := paint(p.color, blue+";"+bold, path)
	if branch != "" {
		header += paint(p.color, dim, " "+branch)
	}
	fmt.Fprintln(stdout, header)
	p.tree(tree.Root, "")
	if *diffRev != "" {
		fmt.Fprintf(stdout, "\n%s changed, +%d -%d\n", plural(tree.Root.Changes, "file"), tree.Root.Added, tree.Root.Deleted)
	} else {
		printSummary(stdout, tree)
	}
	return 0
}

func addChurn(path string, t *Tree, since string) error {
	repo, err := openRepo(path)
	if err != nil {
		return err
	}
	counts, err := repo.churn(since)
	if err != nil {
		return err
	}
	applyChurn(t.Root, counts)
	return nil
}

// load builds the tree for the requested mode: a filesystem walk, optionally
// annotated with git status, or a tree of just the paths in a git diff.
func load(path string, opts Options, gitStatus bool, diffRev string) (*Tree, string, error) {
	if !gitStatus && diffRev == "" {
		t, err := BuildTree(path, opts)
		return t, "", err
	}

	repo, err := openRepo(path)
	if err != nil {
		return nil, "", err
	}
	if diffRev != "" {
		files, err := repo.diff(diffRev)
		if err != nil {
			return nil, "", err
		}
		return diffTree(displayName(path), files, opts.MaxDepth), diffRev, nil
	}

	t, err := BuildTree(path, opts)
	if err != nil {
		return nil, "", err
	}
	changes, err := repo.status()
	if err != nil {
		return nil, "", err
	}
	applyChanges(t, changes)
	return t, "on " + repo.branch(), nil
}

// printSummary mirrors tree's closing line, and says what was left out so
// automatic filtering is never silent.
func printSummary(w io.Writer, t *Tree) {
	dirs, files := t.Root.Count()
	fmt.Fprintf(w, "\n%s, %s", plural(dirs, "directory"), plural(files, "file"))
	if t.Skipped > 0 {
		fmt.Fprintf(w, " (%d hidden or ignored, --all to show)", t.Skipped)
	}
	fmt.Fprintln(w)
}

// parseArgs lets flags appear before or after the path. The flag package
// stops at the first positional argument, so `sprout src --depth 2` would
// otherwise silently ignore --depth.
func parseArgs(fs *flag.FlagSet, args []string) (string, error) {
	path := ""
	for {
		if err := fs.Parse(args); err != nil {
			return "", err
		}
		if fs.NArg() == 0 {
			break
		}
		if path != "" {
			fmt.Fprintf(fs.Output(), "sprout: unexpected argument %q\n", fs.Arg(0))
			return "", fmt.Errorf("unexpected argument %q", fs.Arg(0))
		}
		path = fs.Arg(0)
		args = fs.Args()[1:]
	}
	if path == "" {
		path = "."
	}
	return path, nil
}

// resolveVersion prefers the release ldflag, falling back to the module
// version recorded by `go install ...@vX.Y.Z`.
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return version
}
