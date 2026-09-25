package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

// version is set at release time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("sprout", flag.ContinueOnError)
	fs.SetOutput(stderr)

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
	showVersion := fs.Bool("version", false, "print version and exit")

	path, err := parseArgs(fs, args)
	if err != nil {
		if err == flag.ErrHelp {
			return 0
		}
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
		ShowHidden: *hidden || all,
		MaxDepth:   depth,
		Ignore:     splitPatterns(*ignore),
		NoIgnore:   *noIgnore || all,
	}

	tree, err := BuildTree(path, opts)
	if err != nil {
		fmt.Fprintln(stderr, "sprout:", err)
		return 1
	}

	if *stats {
		PrintStats(stdout, tree, path)
		return 0
	}

	fmt.Fprintln(stdout, path)
	PrintTree(stdout, tree.Root, "", nil)
	printSummary(stdout, tree)
	return 0
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
