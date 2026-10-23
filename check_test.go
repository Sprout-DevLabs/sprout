package main

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	dir := impactRepo(t)
	chdir(t, dir)
	write(t, dir, ".sproutrules", "# layers\nb/ -> a/\nc/ -> a/\n**/y.ts -> web/x.ts\nnope/ -> a/\n")
	out, errOut, code := runCLI(t, "check")
	if code != 1 {
		t.Fatalf("exit %d, want 1: %s%s", code, out, errOut)
	}
	for _, want := range []string{
		"2 imports break .sproutrules (5 files checked)", // b_test.go is a test: left out
		".sproutrules:2  b/ -> a/\n  b/b.go -> a/a.go  uses a.Hello",
		".sproutrules:4  **/y.ts -> web/x.ts\n  web/y.ts -> web/x.ts  imports ./x.js",
		".sproutrules:5  nope/ -> a/ matches no file",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "c/c.go ->") {
		t.Errorf("c reaches a only through b, which isn't an import of its own:\n%s", out)
	}

	// --diff checks only the files a change touched.
	git(t, dir, "add", ".")
	git(t, dir, "commit", "-q", "-m", "rules")
	write(t, dir, "c/c.go", "package c\n\nimport \"ex/b\"\n\nfunc C() string { return b.B() + \"!\" }\n")
	git(t, dir, "commit", "-qam", "c")
	if out, errOut, code := runCLI(t, "check", "--diff", "HEAD~1...HEAD"); code != 0 || !strings.Contains(out, "No imports break the 4 rules in .sproutrules (1 file changed in HEAD~1...HEAD checked)") {
		t.Errorf("--diff: exit %d: %s%s", code, out, errOut)
	}

	write(t, dir, ".sproutrules", "b/ a/\n")
	if _, errOut, code := runCLI(t, "check"); code != 2 || !strings.Contains(errOut, ".sproutrules:1: expected a rule like web/ -> db/") {
		t.Errorf("a malformed rule: exit %d: %s", code, errOut)
	}
}
