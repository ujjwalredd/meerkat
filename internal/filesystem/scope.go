package filesystem

import (
	"os"
	"path/filepath"
	"strings"
)

// Resolve returns the real, absolute, symlink-resolved path.
// For non-existent paths, returns absolute path without symlink resolution.
func Resolve(p string) (string, error) {
	exp := ExpandTilde(p)
	abs, err := filepath.Abs(exp)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return real, nil
}

func ExpandTilde(p string) string {
	if strings.HasPrefix(p, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// Inside reports whether child is inside (or equal to) parent.
// Both should be absolute, symlink-resolved.
func Inside(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if parent == child {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// MatchAny returns true if path matches any glob in patterns. Patterns
// support filepath.Match semantics; bare paths match by prefix.
func MatchAny(path string, patterns []string, projectRoot string) bool {
	abs, err := Resolve(path)
	if err != nil {
		abs = path
	}
	for _, pat := range patterns {
		pe := ExpandTilde(pat)
		if !filepath.IsAbs(pe) {
			pe = filepath.Join(projectRoot, pe)
		}
		pe = filepath.Clean(pe)
		// Resolve symlinks on pattern too so comparisons match `abs`
		// (Resolve falls back to the absolute path if EvalSymlinks fails).
		if rp, err := Resolve(pe); err == nil {
			pe = rp
		}
		// Glob match
		if matched, _ := filepath.Match(pe, abs); matched {
			return true
		}
		// Prefix match (e.g. dir)
		if Inside(pe, abs) {
			return true
		}
	}
	return false
}
