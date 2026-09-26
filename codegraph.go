package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	goprinter "go/printer"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
)

// The code graph gives --ai and --entry what a tree can't: the top-level
// declarations in each source file and which files import which, so the
// most depended-on code can be ranked first (the idea behind aider's repo
// map). Go is parsed with the standard library; other languages use
// line-oriented patterns, which cover the common declaration and import
// forms without a parser dependency.
//
// ponytail: regex extraction misses unusual formatting and path aliases
// (tsconfig "@/..."); tree-sitter would fix both at the cost of cgo.

const (
	maxGraphFiles = 50000
	maxSourceSize = 512 << 10
)

type sourceFile struct {
	rel        string
	symbols    []string
	imports    map[string]bool // resolved repo-relative paths
	importedBy int
}

type codeGraph struct {
	files map[string]*sourceFile
}

// ranked returns files that at least one other file imports, most
// imported first, skipping tests, examples and vendored code.
func (g *codeGraph) ranked() []*sourceFile {
	var out []*sourceFile
	for _, f := range g.files {
		if f.importedBy > 0 && !isTestPath(f.rel) && !isTestFile(f.rel) {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].importedBy != out[j].importedBy {
			return out[i].importedBy > out[j].importedBy
		}
		return out[i].rel < out[j].rel
	})
	return out
}

func isTestFile(rel string) bool {
	base := path.Base(rel)
	return strings.HasSuffix(base, "_test.go") || strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") || strings.HasPrefix(base, "test_")
}

func buildGraph(root string, t *Tree) *codeGraph {
	g := &codeGraph{files: map[string]*sourceFile{}}
	var paths []*Node
	walk(t.Root, func(n *Node) {
		// Tests, vendored code and fixtures never vote or rank, so don't
		// read them at all (in kubernetes that's most of the Go files).
		if !n.IsDir && n.Path != "" && langOf(n.Name) != "" && len(paths) < maxGraphFiles &&
			!isTestPath(n.Rel) && !isTestFile(n.Rel) {
			paths = append(paths, n)
		}
	})
	// Parsing allocates heavily and briefly; trade some memory for less GC.
	defer debug.SetGCPercent(debug.SetGCPercent(400))

	exists := map[string]bool{}
	byDir := map[string][]string{}
	for _, n := range paths {
		exists[n.Rel] = true
		byDir[path.Dir(n.Rel)] = append(byDir[path.Dir(n.Rel)], n.Rel)
	}
	r := resolver{exists: exists, byDir: byDir, goModule: goModulePath(root), javaIndex: javaIndex(paths)}

	// Parse in parallel: reading and parsing dominate on large repos.
	type result struct {
		f    *sourceFile
		info *goInfo
	}
	results := make([]result, len(paths))
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < runtime.NumCPU(); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				results[i].f, results[i].info = parseSource(paths[i], r)
			}
		}()
	}
	for i := range paths {
		next <- i
	}
	close(next)
	wg.Wait()

	goFiles := map[string]goInfo{}
	for _, res := range results {
		if res.f == nil {
			continue
		}
		g.files[res.f.rel] = res.f
		if res.info != nil {
			goFiles[res.f.rel] = *res.info
		}
	}
	linkGoPackages(g, goFiles)
	for _, f := range g.files { // tests were never read, so only real code votes
		for target := range f.imports {
			if tf := g.files[target]; tf != nil {
				tf.importedBy++
			}
		}
	}
	return g
}

// parseSource reads one file and resolves its imports. The resolver is
// read-only, so this is safe to run concurrently.
func parseSource(n *Node, r resolver) (*sourceFile, *goInfo) {
	if n.Size > maxSourceSize {
		return nil, nil
	}
	src, err := os.ReadFile(n.Path)
	if err != nil || bytes.IndexByte(src, 0) >= 0 {
		return nil, nil
	}
	f := &sourceFile{rel: n.Rel, imports: map[string]bool{}}
	var specs []string
	var info *goInfo
	lang := langOf(n.Name)
	if lang == "go" {
		var gi goInfo
		f.symbols, specs, gi = goDecls(n.Path, src)
		info = &gi
	} else {
		f.symbols, specs = scanDecls(lang, src)
	}
	for _, spec := range specs {
		for _, target := range r.resolve(lang, n.Rel, spec) {
			if target != n.Rel {
				f.imports[target] = true
			}
		}
	}
	return f, info
}

func langOf(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go":
		return "go"
	case ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".mts", ".cts":
		return "js"
	case ".py":
		return "py"
	case ".rs":
		return "rs"
	case ".java":
		return "java"
	case ".kt":
		return "kt"
	}
	return ""
}

// ---------- Go ----------

// goInfo is what a Go file declares at top level and which identifiers it
// uses. Files in one package use each other without imports, so these
// references are how a single-package program gets ranked.
type goInfo struct {
	pkg      string
	declares []string
	uses     map[string]bool
}

// goDecls returns exported top-level declarations with their signatures,
// the file's import paths, and its declarations and identifier uses.
func goDecls(filename string, src []byte) (symbols, imports []string, info goInfo) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, info
	}
	info = goInfo{pkg: file.Name.Name, uses: map[string]bool{}}
	ast.Inspect(file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			info.uses[id.Name] = true
		}
		return true
	})
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && d.Name.Name != "main" && d.Name.Name != "init" {
				info.declares = append(info.declares, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					info.declares = append(info.declares, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name != "_" {
							info.declares = append(info.declares, n.Name)
						}
					}
				}
			}
		}
	}
	for _, imp := range file.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	// Exported API first; a program's unexported functions come after,
	// since in package main nothing is exported.
	var private []string
	for _, d := range file.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			sig := &ast.FuncDecl{Recv: d.Recv, Name: d.Name, Type: d.Type} // no body, no doc
			var b bytes.Buffer
			goprinter.Fprint(&b, fset, sig)
			if d.Name.IsExported() {
				symbols = append(symbols, oneLine(b.String()))
			} else if d.Name.Name != "init" {
				private = append(private, oneLine(b.String()))
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				if ts, ok := s.(*ast.TypeSpec); ok {
					decl := "type " + ts.Name.Name + " " + goTypeKind(ts.Type)
					if ts.Name.IsExported() {
						symbols = append(symbols, strings.TrimSpace(decl))
					} else {
						private = append(private, strings.TrimSpace(decl))
					}
				}
			}
		}
	}
	symbols = append(symbols, private...)
	return symbols, imports, info
}

// linkGoPackages adds an edge from each Go file to the files in the same
// directory and package whose top-level names it uses.
func linkGoPackages(g *codeGraph, files map[string]goInfo) {
	owner := map[string]string{} // "dir|pkg|name" -> declaring file
	for rel, info := range files {
		for _, name := range info.declares {
			owner[path.Dir(rel)+"|"+info.pkg+"|"+name] = rel
		}
	}
	for rel, info := range files {
		f := g.files[rel]
		if f == nil || isTestFile(rel) {
			continue
		}
		for name := range info.uses {
			if target, ok := owner[path.Dir(rel)+"|"+info.pkg+"|"+name]; ok && target != rel {
				f.imports[target] = true
			}
		}
	}
}

func goTypeKind(e ast.Expr) string {
	switch e.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.InterfaceType:
		return "interface"
	case *ast.FuncType:
		return "func"
	case *ast.MapType:
		return "map"
	}
	return ""
}

func goModulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if m, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(m), `"`)
		}
	}
	return ""
}

// ---------- other languages ----------

var declPatterns = map[string][]*regexp.Regexp{
	"js": {
		regexp.MustCompile(`^export\s+(?:default\s+)?(?:declare\s+)?(?:abstract\s+)?(?:async\s+)?(?:function\*?|class|interface|type|enum|const|let|var)\s+[A-Za-z_$][\w$]*.*`),
	},
	"py": {
		regexp.MustCompile(`^(?:async\s+)?def\s+[A-Za-z]\w*\s*\(.*`),
		regexp.MustCompile(`^class\s+[A-Za-z]\w*.*`),
	},
	"rs": {
		regexp.MustCompile(`^\s*pub(?:\([^)]*\))?\s+(?:async\s+)?(?:unsafe\s+)?(?:fn|struct|enum|trait|type|const)\s+\w+.*`),
	},
	"java": {
		regexp.MustCompile(`^\s*public\s+(?:(?:final|abstract|static|sealed)\s+)*(?:class|interface|enum|record)\s+\w+`),
	},
	"kt": {
		regexp.MustCompile(`^(?:(?:public|internal|data|sealed|open|abstract|enum)\s+)*(?:class|interface|object|fun)\s+[\w.<>]+.*`),
	},
}

var importPatterns = map[string][]*regexp.Regexp{
	"js": {
		regexp.MustCompile(`(?:import|export)\s[^'"]*?from\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`import\s*\(?\s*['"]([^'"]+)['"]`),
		regexp.MustCompile(`require\(\s*['"]([^'"]+)['"]\s*\)`),
	},
	"py": {
		regexp.MustCompile(`^\s*from\s+(\.*[\w.]*)\s+import\b`),
		regexp.MustCompile(`^\s*import\s+([\w.]+)`),
	},
	"rs": {
		regexp.MustCompile(`^\s*(?:pub\s+)?mod\s+(\w+)\s*;`),
		regexp.MustCompile(`^\s*(?:pub\s+)?use\s+(crate::[\w:]+)`),
	},
	"java": {regexp.MustCompile(`^\s*import\s+(?:static\s+)?([\w.]+)\s*;`)},
	"kt":   {regexp.MustCompile(`^\s*import\s+([\w.]+)`)},
}

// pyFromDot matches "from . import a, b" and "from .. import c": sibling
// modules imported by name.
var pyFromDot = regexp.MustCompile(`^\s*from\s+(\.+)\s+import\s+\(?([\w\s,]+)`)

func scanDecls(lang string, src []byte) (symbols, imports []string) {
	for _, line := range strings.Split(string(src), "\n") {
		if lang == "py" {
			if m := pyFromDot.FindStringSubmatch(line); m != nil {
				for _, name := range strings.Split(m[2], ",") {
					if f := strings.Fields(name); len(f) > 0 {
						imports = append(imports, m[1]+f[0]) // "from . import x as y" -> ".x"
					}
				}
			}
		}
		for _, re := range declPatterns[lang] {
			if m := re.FindString(line); m != "" {
				symbols = append(symbols, trimDecl(m))
				break
			}
		}
		for _, re := range importPatterns[lang] {
			for _, m := range re.FindAllStringSubmatch(line, -1) {
				imports = append(imports, m[1])
			}
		}
	}
	return symbols, imports
}

// trimDecl keeps a declaration's signature and drops its body or value.
func trimDecl(s string) string {
	s = strings.TrimSpace(s)
	for _, cut := range []string{" {", "{", " = ", " =>"} {
		if i := strings.Index(s, cut); i > 0 {
			s = s[:i]
		}
	}
	s = strings.TrimSuffix(strings.TrimSuffix(s, ":"), ";")
	if strings.HasSuffix(s, "(") {
		s += "…)" // parameters continue on the next lines
	}
	return oneLine(s)
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 140 {
		s = s[:140] + "…"
	}
	return s
}

// ---------- import resolution ----------

type resolver struct {
	exists    map[string]bool
	byDir     map[string][]string
	goModule  string
	javaIndex map[string]string // "com/acme/Foo" -> rel path
}

func (r resolver) resolve(lang, from, spec string) []string {
	dir := path.Dir(from)
	switch lang {
	case "go":
		if r.goModule == "" || (spec != r.goModule && !strings.HasPrefix(spec, r.goModule+"/")) {
			return nil
		}
		pkg := strings.TrimPrefix(strings.TrimPrefix(spec, r.goModule), "/")
		if pkg == "" {
			pkg = "."
		}
		var files []string
		for _, f := range r.byDir[pkg] {
			if strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go") {
				files = append(files, f)
			}
		}
		return files
	case "js":
		if !strings.HasPrefix(spec, ".") {
			return nil // a package, not a local file
		}
		base := path.Join(dir, spec)
		for _, cand := range []string{base, base + ".ts", base + ".tsx", base + ".js", base + ".jsx",
			base + ".mjs", base + ".cjs", base + "/index.ts", base + "/index.tsx", base + "/index.js", base + "/index.jsx"} {
			if r.exists[cand] {
				return []string{cand}
			}
		}
		// "./x.js" written in TypeScript source often means x.ts
		if trimmed := strings.TrimSuffix(base, path.Ext(base)); trimmed != base {
			for _, ext := range []string{".ts", ".tsx"} {
				if r.exists[trimmed+ext] {
					return []string{trimmed + ext}
				}
			}
		}
	case "py":
		mod := spec
		base := ""
		if strings.HasPrefix(mod, ".") {
			up := len(mod) - len(strings.TrimLeft(mod, "."))
			base = dir
			for i := 1; i < up; i++ {
				base = path.Dir(base)
			}
			mod = mod[up:]
		}
		rel := strings.ReplaceAll(mod, ".", "/")
		for _, prefix := range []string{base, "", "src"} {
			p := path.Join(prefix, rel)
			for _, cand := range []string{p + ".py", path.Join(p, "__init__.py")} {
				if r.exists[cand] {
					return []string{cand}
				}
			}
		}
	case "rs":
		if name, ok := strings.CutPrefix(spec, "crate::"); ok {
			// crate::a::b::Item -> src/a/b.rs, src/a/b/mod.rs, src/a.rs, ...
			parts := strings.Split(name, "::")
			for i := len(parts); i > 0; i-- {
				p := path.Join("src", strings.Join(parts[:i], "/"))
				for _, cand := range []string{p + ".rs", path.Join(p, "mod.rs")} {
					if r.exists[cand] {
						return []string{cand}
					}
				}
			}
			return nil
		}
		modDir := dir // `mod x;` in main.rs, lib.rs or mod.rs lives next to it
		if b := path.Base(from); b != "main.rs" && b != "lib.rs" && b != "mod.rs" {
			modDir = strings.TrimSuffix(from, ".rs")
		}
		for _, cand := range []string{path.Join(modDir, spec+".rs"), path.Join(modDir, spec, "mod.rs")} {
			if r.exists[cand] {
				return []string{cand}
			}
		}
	case "java", "kt":
		if f, ok := r.javaIndex[strings.ReplaceAll(spec, ".", "/")]; ok {
			return []string{f}
		}
	}
	return nil
}

// javaIndex maps package paths like com/acme/Foo to the file declaring
// them, whatever source root (src/main/java, app/src/...) they live under.
func javaIndex(paths []*Node) map[string]string {
	idx := map[string]string{}
	for _, n := range paths {
		if l := langOf(n.Name); l != "java" && l != "kt" {
			continue
		}
		p := strings.TrimSuffix(n.Rel, path.Ext(n.Rel))
		// index every suffix, so com/acme/Foo matches src/main/java/com/acme/Foo
		for i := 0; i < len(p); i++ {
			if i == 0 || p[i-1] == '/' {
				if _, taken := idx[p[i:]]; !taken {
					idx[p[i:]] = n.Rel
				}
			}
		}
	}
	return idx
}
