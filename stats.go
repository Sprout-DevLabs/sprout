package main

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
)

// Stats is the aggregate result of walking a tree for --stats.
type Stats struct {
	Files       int
	Directories int
	TotalSize   int64
	Languages   map[string]int
}

// languageNames maps file extensions to a human-readable language label.
// Extensions not listed here fall back to their uppercased form (e.g. .toml -> TOML).
var languageNames = map[string]string{
	".go":   "Go",
	".py":   "Python",
	".js":   "JavaScript",
	".jsx":  "JavaScript",
	".ts":   "TypeScript",
	".tsx":  "TypeScript",
	".css":  "CSS",
	".scss": "SCSS",
	".md":   "Markdown",
	".json": "JSON",
	".yaml": "YAML",
	".yml":  "YAML",
	".html": "HTML",
	".java": "Java",
	".c":    "C",
	".h":    "C Header",
	".cpp":  "C++",
	".rs":   "Rust",
	".rb":   "Ruby",
	".php":  "PHP",
	".sh":   "Shell",
	".sql":  "SQL",
}

// fileNames covers well-known files that carry no extension.
var fileNames = map[string]string{
	"Makefile":   "Makefile",
	"Dockerfile": "Dockerfile",
	"Justfile":   "Justfile",
	"Gemfile":    "Ruby",
	"Rakefile":   "Ruby",
	"go.mod":     "Go modules",
	"go.sum":     "Go modules",
}

// languageOf returns a display label for a file, or "" if it has none.
func languageOf(name string) string {
	if lang, ok := fileNames[name]; ok {
		return lang
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" || ext == name { // no extension, or a dotfile like .env
		return ""
	}
	if lang, ok := languageNames[ext]; ok {
		return lang
	}
	return strings.ToUpper(strings.TrimPrefix(ext, "."))
}

func collectStats(node *Node, s *Stats) {
	for _, child := range node.Children {
		if child.IsDir {
			s.Directories++
			collectStats(child, s)
			continue
		}

		s.Files++
		s.TotalSize += child.Size

		if lang := languageOf(child.Name); lang != "" {
			s.Languages[lang]++
		}
	}
}

// formatSize labels binary units honestly (KiB, MiB); si gives kB, MB.
func formatSize(bytes int64, si bool) string {
	unit, units := int64(1024), []string{"KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	if si {
		unit, units = 1000, []string{"kB", "MB", "GB", "TB", "PB", "EB"}
	}
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := unit, 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}

// PrintStats renders the my-project/ summary shown in the README's --stats example.
func PrintStats(w io.Writer, t *Tree, path string, si bool) {
	s := &Stats{Languages: make(map[string]int)}
	collectStats(t.Root, s)

	fmt.Fprintf(w, "%s/\n", displayName(path))
	for _, p := range DetectProject(path) {
		fmt.Fprintf(w, "Project:        %s (%s, %s)\n", p.Language, p.PackageManager, p.Manifest)
	}
	fmt.Fprintf(w, "Files:          %d\n", s.Files)
	fmt.Fprintf(w, "Directories:    %d\n", s.Directories)
	fmt.Fprintf(w, "Total size:     %s\n", formatSize(s.TotalSize, si))
	if t.Skipped > 0 {
		fmt.Fprintf(w, "Ignored:        %d entries (--all to include)\n", t.Skipped)
	}

	if len(s.Languages) == 0 {
		return
	}

	type langCount struct {
		Name  string
		Count int
	}
	var langs []langCount
	for name, count := range s.Languages {
		langs = append(langs, langCount{name, count})
	}
	sort.Slice(langs, func(i, j int) bool {
		if langs[i].Count != langs[j].Count {
			return langs[i].Count > langs[j].Count
		}
		return langs[i].Name < langs[j].Name
	})

	fmt.Fprintln(w, "Languages:")
	for _, l := range langs {
		fmt.Fprintf(w, "  %-14s %s\n", l.Name, plural(l.Count, "file"))
	}
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	if strings.HasSuffix(word, "y") {
		return fmt.Sprintf("%d %sies", n, strings.TrimSuffix(word, "y"))
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// displayName turns "." or "../x/" into the directory's real name.
func displayName(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return filepath.Base(abs)
	}
	return path
}
