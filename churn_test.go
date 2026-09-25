package main

import (
	"strings"
	"testing"
)

func TestChurn(t *testing.T) {
	dir := setupGitRepo(t)
	for i, content := range []string{"1", "2", "3"} {
		write(t, dir, "src/hot.go", content)
		if i == 0 {
			write(t, dir, "src/cold.go", "x")
		}
		git(t, dir, "add", "-A")
		git(t, dir, "commit", "-qm", "c"+content)
	}

	out, errOut, code := runCLI(t, dir, "--churn")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"hot.go  █ 3", "cold.go  ▃ 1", "src/  █ 3"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestChurnBar(t *testing.T) {
	cases := []struct {
		n, max int
		want   string
	}{
		{0, 10, ""}, {1, 8, "▁ 1"}, {8, 8, "█ 8"}, {5, 10, "▄ 5"},
	}
	for _, c := range cases {
		if got := churnBar(c.n, c.max); got != c.want {
			t.Errorf("churnBar(%d, %d) = %q, want %q", c.n, c.max, got, c.want)
		}
	}
}
