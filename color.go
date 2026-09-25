package main

import (
	"io"
	"os"
)

// useColor follows the NO_COLOR convention (https://no-color.org) and only
// colors real terminals, so pipes and files get plain text.
func useColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
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
