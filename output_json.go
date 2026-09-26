package main

import (
	"bytes"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"
)

// jsonSchemaVersion is bumped on any breaking change to the --json shape.
// Adding fields is not breaking.
const jsonSchemaVersion = 1

// The --json output is written by a small streaming writer instead of
// encoding/json. Each node used to marshal itself with json.Marshal, which
// built a temporary buffer per node that its parent then copied, and the
// encoder then re-indented the whole document: on llvm-project (185k files)
// that was ~1.2 GB of allocation. The writer below encodes every value once,
// straight into one reusable buffer.
//
// It produces byte-for-byte what encoding/json produced (same field order,
// omitempty rules, sorted map keys, string escaping, and Encoder.SetIndent
// layout); output_json_compat_test.go checks that against encoding/json itself.

type jsonWriter struct {
	w      io.Writer
	buf    []byte
	err    error
	indent bool
	depth  int
	empty  bool // no element written yet in the innermost open container
}

func newJSONWriter(w io.Writer, indent bool) *jsonWriter {
	return &jsonWriter{w: w, buf: make([]byte, 0, 64<<10), indent: indent}
}

// flushIfFull hands full chunks to the underlying writer, so memory use
// stays flat however large the tree is.
func (j *jsonWriter) flushIfFull() {
	if len(j.buf) >= 32<<10 {
		j.flush()
	}
}

func (j *jsonWriter) flush() error {
	if j.err == nil && len(j.buf) > 0 {
		_, j.err = j.w.Write(j.buf)
	}
	j.buf = j.buf[:0]
	return j.err
}

func (j *jsonWriter) newline() {
	j.buf = append(j.buf, '\n')
	for i := 0; i < j.depth; i++ {
		j.buf = append(j.buf, ' ', ' ')
	}
}

func (j *jsonWriter) open(c byte) {
	j.buf = append(j.buf, c)
	j.depth++
	j.empty = true
}

func (j *jsonWriter) close(c byte) {
	j.depth--
	if j.indent && !j.empty {
		j.newline()
	}
	j.buf = append(j.buf, c)
	j.empty = false
}

// elem starts an array element or object member.
func (j *jsonWriter) elem() {
	if !j.empty {
		j.buf = append(j.buf, ',')
	}
	j.empty = false
	if j.indent {
		j.newline()
	}
}

func (j *jsonWriter) key(k string) {
	j.elem()
	j.buf = appendJSONString(j.buf, k)
	j.buf = append(j.buf, ':')
	if j.indent {
		j.buf = append(j.buf, ' ')
	}
}

func (j *jsonWriter) str(k, v string)       { j.key(k); j.buf = appendJSONString(j.buf, v) }
func (j *jsonWriter) int(k string, v int64) { j.key(k); j.buf = strconv.AppendInt(j.buf, v, 10) }
func (j *jsonWriter) bool(k string, v bool) { j.key(k); j.buf = strconv.AppendBool(j.buf, v) }

// node writes one tree node and its children, with the fields, order and
// omitempty behaviour of the original struct-based encoding.
func (j *jsonWriter) node(n *Node) {
	j.open('{')
	j.str("name", n.Name)
	if n.Rel != "" {
		j.str("path", n.Rel)
	}
	if n.IsDir {
		j.str("type", "directory")
	} else {
		j.str("type", "file")
	}
	if n.Size != 0 {
		j.int("size", n.Size)
	}
	if !n.ModTime.IsZero() {
		j.key("modified")
		j.buf = append(j.buf, '"')
		j.buf = n.ModTime.UTC().AppendFormat(j.buf, time.RFC3339) // RFC 3339 needs no escaping
		j.buf = append(j.buf, '"')
	}
	if n.Err != nil {
		if msg := errReason(n.Err); msg != "" {
			j.str("error", msg)
		}
	}
	if n.Truncated {
		j.bool("truncated", true)
	}
	if n.Status != "" {
		j.str("status", n.Status)
	}
	for _, f := range [...]struct {
		k string
		v int
	}{{"changes", n.Changes}, {"added", n.Added}, {"deleted", n.Deleted}, {"churn", n.Churn}, {"moreFiles", n.More}} {
		if f.v != 0 {
			j.int(f.k, int64(f.v))
		}
	}
	if len(n.Children) > 0 {
		j.key("children")
		j.open('[')
		for _, c := range n.Children {
			j.elem()
			j.node(c)
			j.flushIfFull()
		}
		j.close(']')
	}
	j.close('}')
}

// MarshalJSON keeps json.Marshal(node) working, via the same writer.
func (n *Node) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	j := newJSONWriter(&b, false)
	j.node(n)
	if err := j.flush(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WriteJSON writes the --json report: compact by default, indented with
// pretty.
func WriteJSON(w io.Writer, t *Tree, path string, pretty bool) error {
	s := &Stats{Languages: map[string]int{}}
	collectStats(t.Root, s)

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	j := newJSONWriter(w, pretty)
	j.open('{')
	j.int("schemaVersion", jsonSchemaVersion)
	j.str("name", filepath.Base(abs))
	j.str("root", filepath.ToSlash(abs))
	j.bool("gitAware", t.GitAware)

	j.key("projects") // always an array, never null, so consumers can iterate blindly
	j.open('[')
	for _, p := range DetectProject(path) {
		j.elem()
		j.open('{')
		j.str("manifest", p.Manifest)
		j.str("language", p.Language)
		j.str("packageManager", p.PackageManager)
		j.close('}')
	}
	j.close(']')

	j.key("summary")
	j.open('{')
	j.int("directories", int64(s.Directories))
	j.int("files", int64(s.Files))
	j.int("bytes", s.TotalSize)
	j.int("skipped", int64(t.Skipped))
	j.key("languages")
	j.open('{')
	langs := make([]string, 0, len(s.Languages))
	for lang := range s.Languages {
		langs = append(langs, lang)
	}
	sort.Strings(langs) // encoding/json sorts map keys
	for _, lang := range langs {
		j.int(lang, int64(s.Languages[lang]))
	}
	j.close('}')
	j.close('}')

	j.key("tree")
	j.node(t.Root)
	j.close('}')
	j.buf = append(j.buf, '\n') // json.Encoder ends each value with a newline
	return j.flush()
}

// appendJSONString appends s as a JSON string, escaped exactly as
// encoding/json does by default: \" \\ \b \f \n \r \t, other control
// characters and the HTML-sensitive < > & as \u00XX, invalid UTF-8 as
// �, and U+2028/U+2029 as  / .
func appendJSONString(dst []byte, s string) []byte {
	const hex = "0123456789abcdef"
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b >= 0x20 && b != '"' && b != '\\' && b != '<' && b != '>' && b != '&' {
				i++
				continue
			}
			dst = append(dst, s[start:i]...)
			switch b {
			case '"', '\\':
				dst = append(dst, '\\', b)
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hex[b>>4], hex[b&0xF])
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			dst = append(dst, s[start:i]...)
			dst = append(dst, "\\ufffd"...) // the escape sequence, not the character
			i += size
			start = i
			continue
		}
		if c == ' ' || c == ' ' {
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hex[c&0xF])
			i += size
			start = i
			continue
		}
		i += size
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}
