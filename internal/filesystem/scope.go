package filesystem

import (
	"os"
	"path/filepath"
	"strings"
)

// Resolve returns the real, absolute, symlink-resolved path.
// For non-existent paths, walks up to the nearest existing ancestor,
// resolves that, then re-joins the missing tail. This makes path-scope
// checks correct on macOS where /tmp is a symlink to /private/tmp.
func Resolve(p string) (string, error) {
	exp := ExpandTilde(p)
	abs, err := filepath.Abs(exp)
	if err != nil {
		return "", err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real, nil
	}
	// Walk up to nearest existing ancestor.
	dir := abs
	tail := ""
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs, nil
		}
		if _, err := os.Stat(parent); err == nil {
			real, err := filepath.EvalSymlinks(parent)
			if err != nil {
				return abs, nil
			}
			return filepath.Join(real, filepath.Base(dir), tail), nil
		}
		tail = filepath.Join(filepath.Base(dir), tail)
		dir = parent
	}
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
