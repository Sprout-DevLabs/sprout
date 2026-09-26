package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSizeTotalsPastDepth(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "big/deep/er/blob.bin", strings.Repeat("x", 3000))
	write(t, dir, "big/a.txt", strings.Repeat("x", 1000))
	write(t, dir, "small.txt", "hi")

	out, _, code := runCLI(t, dir, "--size", "-L", "1")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	// big/ is collapsed by -L 1 but its total still counts everything below.
	if !strings.Contains(out, "big/  3.9K") || !strings.Contains(out, "small.txt  2B") {
		t.Errorf("sizes:\n%s", out)
	}
	if strings.Contains(out, "blob.bin") {
		t.Errorf("--size must not expand past --depth:\n%s", out)
	}
	if !strings.Contains(out, "1 directory, 1 file") {
		t.Errorf("summary should count only what's shown:\n%s", out)
	}

	out, _, _ = runCLI(t, dir, "--size", "--si", "-L", "1")
	if !strings.Contains(out, "big/  4K") {
		t.Errorf("--si should use powers of 1000:\n%s", out)
	}
}

func TestSortOrders(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", strings.Repeat("x", 10))
	write(t, dir, "b.txt", strings.Repeat("x", 300))
	write(t, dir, "c/x.txt", strings.Repeat("x", 100))
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(filepath.Join(dir, "b.txt"), old, old)

	order := func(args ...string) string {
		out, _, code := runCLI(t, append([]string{dir}, args...)...)
		if code != 0 {
			t.Fatalf("%v: exit %d", args, code)
		}
		var names []string
		for _, l := range strings.Split(out, "\n") {
			if i := strings.Index(l, "── "); i >= 0 {
				names = append(names, strings.Fields(l[i+len("── "):])[0])
			}
		}
		return strings.Join(names, " ")
	}

	cases := map[string][]string{
		"b.txt c/ x.txt a.txt": {"--sort", "size"},
		"a.txt c/ x.txt b.txt": {"--sort", "size", "-r"},
		"c/ x.txt a.txt b.txt": {"--dirs-first"},
		"b.txt":                {"--sort", "time", "-r", "-L", "1"}, // oldest first, even with c/ collapsed
	}
	for want, args := range cases {
		if got := order(args...); !strings.HasPrefix(got, want) {
			t.Errorf("%v: got %q, want prefix %q", args, got, want)
		}
	}
	if _, errOut, code := runCLI(t, dir, "--sort", "color"); code != 2 || !strings.Contains(errOut, "--sort") {
		t.Errorf("bad --sort: exit %d %s", code, errOut)
	}
}

func TestHumanSize(t *testing.T) {
	cases := []struct {
		n    int64
		si   bool
		want string
	}{
		{0, false, "0B"}, {1023, false, "1023B"}, {1024, false, "1K"}, {1536, false, "1.5K"},
		{5 << 20, false, "5M"}, {1000, true, "1K"}, {1500000, true, "1.5M"},
	}
	for _, c := range cases {
		if got := humanSize(c.n, c.si); got != c.want {
			t.Errorf("humanSize(%d, %v) = %q, want %q", c.n, c.si, got, c.want)
		}
	}
}
