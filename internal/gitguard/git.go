package gitguard

import (
	"os/exec"
	"strings"
)

// IsRepo returns true if cwd is a git repository.
func IsRepo() bool {
	return exec.Command("git", "rev-parse", "--is-inside-work-tree").Run() == nil
}

func CurrentBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err == nil {
		if b := strings.TrimSpace(string(out)); b != "" && b != "HEAD" {
			return b
		}
	}
	// Unborn HEAD (no commits yet): read symbolic ref directly.
	out, err = exec.Command("git", "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ChangedFiles returns paths reported by `git status --porcelain` (relative to repo root).
func ChangedFiles() ([]string, error) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if len(l) < 4 {
			continue
		}
		path := strings.TrimSpace(l[3:])
		// handle "old -> new"
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		files = append(files, path)
	}
	return files, nil
}

// StagedFiles returns paths staged for commit.
func StagedFiles() ([]string, error) {
	out, err := exec.Command("git", "diff", "--cached", "--name-only").Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

func RepoRoot() string {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
