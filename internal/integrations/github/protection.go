// Package github queries GitHub branch-protection status so the classifier
// can treat remote-protected branches as protected even if the local
// policy did not list them.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	protected bool
	at        time.Time
}

var (
	cacheMu  sync.Mutex
	cache    = map[string]cacheEntry{}
	cacheTTL = time.Hour
)

// IsProtected returns true if {owner}/{repo}:{branch} has GitHub branch
// protection enabled. Empty token → return (false, nil) (caller decides).
// On API error, returns (false, err); caller should fail open with a
// warning, NEVER silently allow a push.
func IsProtected(ctx context.Context, owner, repo, branch, token string) (bool, error) {
	key := owner + "/" + repo + "#" + branch
	cacheMu.Lock()
	if e, ok := cache[key]; ok && time.Since(e.at) < cacheTTL {
		cacheMu.Unlock()
		return e.protected, nil
	}
	cacheMu.Unlock()

	if token == "" {
		return false, nil
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches/%s/protection",
		owner, repo, branch)
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	c := &http.Client{Timeout: 5 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return false, fmt.Errorf("github: %w", err)
	}
	defer resp.Body.Close()
	protected := false
	switch resp.StatusCode {
	case 200:
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		protected = true
	case 404:
		protected = false
	default:
		return false, fmt.Errorf("github: HTTP %d", resp.StatusCode)
	}
	cacheMu.Lock()
	cache[key] = cacheEntry{protected: protected, at: time.Now()}
	cacheMu.Unlock()
	return protected, nil
}

// ParseRemote parses an `origin` remote URL into owner/repo.
// Accepts https://github.com/<owner>/<repo>(.git) and
// git@github.com:<owner>/<repo>(.git).
func ParseRemote(url string) (owner, repo string, ok bool) {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, ".git")
	if strings.HasPrefix(url, "git@github.com:") {
		s := strings.TrimPrefix(url, "git@github.com:")
		parts := strings.SplitN(s, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1], true
		}
	}
	for _, p := range []string{"https://github.com/", "http://github.com/"} {
		if strings.HasPrefix(url, p) {
			s := strings.TrimPrefix(url, p)
			parts := strings.SplitN(s, "/", 2)
			if len(parts) == 2 {
				return parts[0], parts[1], true
			}
		}
	}
	return "", "", false
}

// DetectOriginOwnerRepo runs `git remote get-url origin` and parses it.
func DetectOriginOwnerRepo() (string, string, bool) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", "", false
	}
	return ParseRemote(string(out))
}

// TokenFromEnv reads the configured env var (default GITHUB_TOKEN).
func TokenFromEnv(name string) string {
	if name == "" {
		name = "GITHUB_TOKEN"
	}
	return os.Getenv(name)
}
