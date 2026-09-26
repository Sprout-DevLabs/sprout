package main

import (
	"strings"
	"testing"
)

func graphFor(t *testing.T, files map[string]string) *codeGraph {
	t.Helper()
	dir := t.TempDir()
	for rel, src := range files {
		write(t, dir, rel, src)
	}
	tree, err := BuildTree(dir, Options{MaxDepth: -1, ShowHidden: true})
	if err != nil {
		t.Fatal(err)
	}
	return buildGraph(dir, tree)
}

func usedBy(g *codeGraph, rel string) int {
	if f := g.files[rel]; f != nil {
		return f.importedBy
	}
	return -1
}

func TestGoGraph(t *testing.T) {
	g := graphFor(t, map[string]string{
		"go.mod":              "module example.com/app\n",
		"main.go":             "package main\nimport \"example.com/app/store\"\nfunc main() { store.Open(); helper() }\n",
		"util.go":             "package main\nfunc helper() {}\n",
		"store/store.go":      "package store\n// Open opens it.\nfunc Open() (*DB, error) { return nil, nil }\ntype DB struct{}\nfunc (d *DB) Get(id string) ([]byte, error) { return nil, nil }\nfunc private() {}\n",
		"store/store_test.go": "package store\nimport \"testing\"\nfunc TestX(t *testing.T) { Open() }\n",
	})
	if usedBy(g, "store/store.go") != 1 || usedBy(g, "util.go") != 1 {
		t.Errorf("store.go used by %d, util.go used by %d; want 1 and 1", usedBy(g, "store/store.go"), usedBy(g, "util.go"))
	}
	syms := strings.Join(g.files["store/store.go"].symbols, "; ")
	for _, want := range []string{"func Open() (*DB, error)", "type DB struct", "func (d *DB) Get(id string) ([]byte, error)", "func private()"} {
		if !strings.Contains(syms, want) {
			t.Errorf("missing %q in %q", want, syms)
		}
	}
	if strings.Index(syms, "func private()") < strings.Index(syms, "type DB struct") {
		t.Errorf("exported declarations should come first: %q", syms)
	}
}

func TestJSGraph(t *testing.T) {
	g := graphFor(t, map[string]string{
		"src/index.ts":       "import { get } from './lib/http'\nexport * from './types'\nexport default function ky(input: string) {}\n",
		"src/lib/http.ts":    "import type { Options } from '../types'\nexport async function get(url: string, opts?: Options): Promise<Response> {\n}\n",
		"src/types/index.ts": "export interface Options { retry: number }\nexport type Method = 'get'\n",
		"src/app.jsx":        "const x = require('./lib/http.js')\nimport('./types')\nimport React from 'react'\n",
		"test/http.test.ts":  "import { get } from '../src/lib/http'\n",
	})
	if n := usedBy(g, "src/types/index.ts"); n != 3 {
		t.Errorf("types used by %d, want 3 (index, http, app; tests don't count)", n)
	}
	if n := usedBy(g, "src/lib/http.ts"); n != 2 {
		t.Errorf("http.ts used by %d, want 2 (.js specifier resolves to .ts)", n)
	}
	syms := strings.Join(g.files["src/lib/http.ts"].symbols, "; ")
	if !strings.Contains(syms, "export async function get(url: string, opts?: Options): Promise<Response>") {
		t.Errorf("signature: %q", syms)
	}
}

func TestPythonGraph(t *testing.T) {
	g := graphFor(t, map[string]string{
		"src/pkg/__init__.py": "from .core import run\n",
		"src/pkg/core.py":     "from . import utils\nfrom pkg.models import User\n\ndef run(argv: list[str]) -> int:\n    pass\n\ndef long_one(\n    a,\n):\n    pass\n\ndef _private():\n    pass\n\nclass Runner(Base):\n    def method(self):\n        pass\n",
		"src/pkg/utils.py":    "import os\n",
		"src/pkg/models.py":   "class User:\n    pass\n",
	})
	for rel, want := range map[string]int{"src/pkg/core.py": 1, "src/pkg/utils.py": 1, "src/pkg/models.py": 1} {
		if n := usedBy(g, rel); n != want {
			t.Errorf("%s used by %d, want %d", rel, n, want)
		}
	}
	syms := strings.Join(g.files["src/pkg/core.py"].symbols, "; ")
	if !strings.Contains(syms, "def run(argv: list[str]) -> int") || !strings.Contains(syms, "def long_one(…)") ||
		!strings.Contains(syms, "class Runner(Base)") || strings.Contains(syms, "_private") || strings.Contains(syms, "method") {
		t.Errorf("python symbols: %q", syms)
	}
}

func TestRustAndJavaGraph(t *testing.T) {
	g := graphFor(t, map[string]string{
		"src/main.rs":                                "mod config;\nmod net;\nuse crate::net::client::Client;\nfn main() {}\n",
		"src/config.rs":                              "pub struct Config { pub port: u16 }\npub fn load(path: &str) -> Config {\n}\nfn hidden() {}\n",
		"src/net/mod.rs":                             "pub mod client;\n",
		"src/net/client.rs":                          "pub struct Client;\n",
		"app/src/main/java/com/acme/App.java":        "package com.acme;\nimport com.acme.store.Repo;\npublic class App {}\n",
		"app/src/main/java/com/acme/store/Repo.java": "package com.acme.store;\npublic final class Repo {}\n",
	})
	for rel, want := range map[string]int{"src/config.rs": 1, "src/net/mod.rs": 1, "src/net/client.rs": 2,
		"app/src/main/java/com/acme/store/Repo.java": 1} {
		if n := usedBy(g, rel); n != want {
			t.Errorf("%s used by %d, want %d", rel, n, want)
		}
	}
	syms := strings.Join(g.files["src/config.rs"].symbols, "; ")
	if !strings.Contains(syms, "pub fn load(path: &str) -> Config") || strings.Contains(syms, "hidden") {
		t.Errorf("rust symbols: %q", syms)
	}
}

func TestEntryAndAIKeyFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "README.md", "# demo\n\nA demo.\n")
	write(t, dir, "go.mod", "module example.com/demo\n")
	write(t, dir, "cmd/demo/main.go", "package main\nimport \"example.com/demo/core\"\nfunc main() { core.Run() }\n")
	write(t, dir, "core/core.go", "package core\nfunc Run() error { return nil }\n")
	write(t, dir, "web/main.go", "package main\nimport \"example.com/demo/core\"\nfunc main() { core.Run() }\n")

	out, _, code := runCLI(t, dir, "--entry")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	for _, want := range []string{"1. README.md", "cmd/demo/main.go", "entry point", "core/core.go", "used by 2 files"} {
		if !strings.Contains(out, want) {
			t.Errorf("--entry missing %q:\n%s", want, out)
		}
	}

	out, _, _ = runCLI(t, dir, "--ai")
	if !strings.Contains(out, "## key files (most used first)\ncore/core.go (used by 2): func Run() error") {
		t.Errorf("--ai key files:\n%s", out)
	}
}
