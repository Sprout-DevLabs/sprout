package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestMatcher(t *testing.T) {
	cases := []struct {
		patterns []string
		rel      string
		isDir    bool
		want     bool
	}{
		{[]string{"*.log"}, "debug.log", false, true},
		{[]string{"*.log"}, "a/b/debug.log", false, true}, // no slash: any depth
		{[]string{"node_modules"}, "web/node_modules", true, true},
		{[]string{"build/"}, "build", false, false}, // dir-only
		{[]string{"build/"}, "src/build", true, true},
		{[]string{"src/gen"}, "src/gen", true, true}, // slash: anchored to root
		{[]string{"src/gen"}, "lib/src/gen", true, false},
		{[]string{"/TODO.md"}, "TODO.md", false, true},
		{[]string{"/TODO.md"}, "docs/TODO.md", false, false},
		{[]string{"docs/**/*.png"}, "docs/a/b/x.png", false, true},
		{[]string{"docs/**/*.png"}, "docs/x.png", false, true},
		{[]string{"docs/**/*.png"}, "img/x.png", false, false},
		{[]string{"**/fixtures"}, "a/b/fixtures", true, true},
		{[]string{"src/*.go"}, "src/a/b.go", false, false}, // * stays in one segment
		{[]string{"*.log", "!keep.log"}, "keep.log", false, false},
		{[]string{"*.log", "!keep.log"}, "other.log", false, true},
		{[]string{"file[0-9].txt"}, "file7.txt", false, true},
		{[]string{"file[!0-9].txt"}, "file7.txt", false, false},
		{[]string{"# comment", "", "  "}, "anything", false, false},
		{[]string{"a+b(c).txt"}, "a+b(c).txt", false, true}, // regex metacharacters are literal
	}
	for _, c := range cases {
		name := c.rel[strings.LastIndex(c.rel, "/")+1:]
		if got := compile(c.patterns).match(c.rel, name, c.isDir); got != c.want {
			t.Errorf("%q matching %q (dir=%v) = %v, want %v", c.patterns, c.rel, c.isDir, got, c.want)
		}
	}
}

func TestPatternListFlag(t *testing.T) {
	var p patternList
	p.Set(" foo , bar,,")
	p.Set("baz")
	if want := (patternList{"foo", "bar", "baz"}); !reflect.DeepEqual(p, want) {
		t.Errorf("patternList = %v, want %v", p, want)
	}
}

func TestOnlyAndSproutignore(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"src/app/main.go", "src/app/main_test.go", "src/app/style.css",
		"web/index.ts", "docs/guide.md", "gen/api.pb.go", "notes.txt"} {
		write(t, dir, f, "x")
	}
	write(t, dir, ".sproutignore", "# generated code\ngen/\n*_test.go\n")

	out, _, code := runCLI(t, dir, "--only", "*.go,*.ts")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"main.go", "index.ts"} {
		if !strings.Contains(out, want) {
			t.Errorf("--only dropped %s:\n%s", want, out)
		}
	}
	for _, gone := range []string{"style.css", "docs", "notes.txt", "gen", "main_test.go"} {
		if strings.Contains(out, gone) {
			t.Errorf("%s should be filtered (by --only, pruning, or .sproutignore):\n%s", gone, out)
		}
	}

	out, _, _ = runCLI(t, dir, "--ignore", "src/app/", "--ignore", "*.md")
	if strings.Contains(out, "app/") || strings.Contains(out, "guide.md") || !strings.Contains(out, "web/") {
		t.Errorf("path and name --ignore patterns:\n%s", out)
	}

	out, _, _ = runCLI(t, dir, "--no-ignore")
	if !strings.Contains(out, "api.pb.go") {
		t.Errorf("--no-ignore should skip .sproutignore:\n%s", out)
	}
}
