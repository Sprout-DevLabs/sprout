package main

import (
	"io"
	"os"
	"testing"
)

// Benchmarks against a real repository, e.g.:
//
//	git clone --depth 1 https://github.com/kubernetes/kubernetes /tmp/k8s
//	SPROUT_BENCH_DIR=/tmp/k8s go test -run '^$' -bench .
func benchRun(b *testing.B, args ...string) {
	dir := os.Getenv("SPROUT_BENCH_DIR")
	if dir == "" {
		b.Skip("set SPROUT_BENCH_DIR to a large repository")
	}
	for i := 0; i < b.N; i++ {
		if code := run(append([]string{dir, "--no-config"}, args...), io.Discard, io.Discard); code != 0 {
			b.Fatalf("exit %d", code)
		}
	}
}

func BenchmarkTree(b *testing.B)       { benchRun(b) }
func BenchmarkTreeDepth2(b *testing.B) { benchRun(b, "-L", "2") }
func BenchmarkJSON(b *testing.B)       { benchRun(b, "--json") }
func BenchmarkSize(b *testing.B)       { benchRun(b, "--size", "-L", "1") }
func BenchmarkEntry(b *testing.B)      { benchRun(b, "--entry") }
func BenchmarkAI(b *testing.B)         { benchRun(b, "--ai") }
