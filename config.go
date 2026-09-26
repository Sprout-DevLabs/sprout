package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config files hold default flags, one per line, like ripgrep's:
//
//	# ~/.config/sprout/config
//	--depth 3
//	--ignore=*.snap,fixtures/
//	--budget 4000
//
// The user file is read first, then the nearest .sproutrc from the target
// directory upwards, then the command line, so later settings win and
// --ignore/--only patterns accumulate. --no-config skips both files.

// userConfigPath honours $SPROUT_CONFIG, then $XDG_CONFIG_HOME, then
// ~/.config on Unix and macOS (where developers expect it) and %AppData%
// on Windows.
func userConfigPath() string {
	if p := os.Getenv("SPROUT_CONFIG"); p != "" {
		return p
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "sprout", "config")
	}
	if a := os.Getenv("AppData"); a != "" {
		return filepath.Join(a, "sprout", "config")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "sprout", "config")
}

// projectConfigPath finds the nearest .sproutrc at or above dir.
func projectConfigPath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for {
		p := filepath.Join(abs, ".sproutrc")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return ""
		}
		abs = parent
	}
}

// loadConfig returns the flags from the user and project config files.
func loadConfig(dir string) ([]string, error) {
	var args []string
	project := ""
	if dir != "" {
		project = projectConfigPath(dir)
	}
	for _, p := range []string{userConfigPath(), project} {
		if p == "" {
			continue
		}
		a, err := readConfig(p)
		if err != nil {
			return nil, err
		}
		args = append(args, a...)
	}
	return args, nil
}

func readConfig(p string) ([]string, error) {
	f, err := os.Open(p)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var args []string
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "-") {
			return nil, fmt.Errorf("%s:%d: expected a flag like --depth 3, got %q", p, n, line)
		}
		// "--flag=value" is one argument even if the value has spaces;
		// "--flag value" is two.
		if strings.Contains(line, "=") {
			args = append(args, line)
		} else {
			args = append(args, strings.Fields(line)...)
		}
	}
	return args, sc.Err()
}
