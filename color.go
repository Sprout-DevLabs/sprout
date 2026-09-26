package main

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// useColor follows the NO_COLOR convention (https://no-color.org) and only
// colors real terminals, so pipes and files get plain text.
func useColor(w io.Writer) bool {
	return os.Getenv("NO_COLOR") == "" && isTerminal(w)
}

func isTerminal(w io.Writer) bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

const (
	bold   = "1"
	dim    = "2"
	red    = "31"
	green  = "32"
	yellow = "33"
	blue   = "34"
	cyan   = "36"
)

func paint(on bool, code, s string) string {
	if !on || s == "" {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

var statusColor = map[string]string{
	"M": yellow, "A": green, "?": green, "D": red, "R": cyan, "U": red + ";" + bold,
}

// hyperlink wraps text in an OSC 8 link to a local file, which terminals
// like iTerm2, WezTerm, Kitty, GNOME Terminal and Windows Terminal make
// clickable. Terminals without support show the text unchanged.
func hyperlink(text, path, host string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return text
	}
	p := filepath.ToSlash(abs)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p // C:/x -> /C:/x
	}
	u := url.URL{Scheme: "file", Host: host, Path: p}
	return "\x1b]8;;" + u.String() + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}
