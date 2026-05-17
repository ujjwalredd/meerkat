package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/filesystem"
)

type Finding struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Type   string `json:"type"`
	Redact string `json:"redacted"`
}

type rule struct {
	name string
	re   *regexp.Regexp
}

var rules = []rule{
	{"aws_access_key", regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"github_token", regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{30,}\b`)},
	{"openai_api_key", regexp.MustCompile(`\bsk-[A-Za-z0-9_\-]{20,}\b`)},
	{"anthropic_api_key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_\-]{20,}\b`)},
	{"slack_token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`)},
	{"stripe_key", regexp.MustCompile(`\b(sk|pk|rk)_(live|test)_[A-Za-z0-9]{20,}\b`)},
	{"private_key", regexp.MustCompile(`-----BEGIN (RSA|EC|DSA|OPENSSH|PGP|PRIVATE) (PRIVATE )?KEY-----`)},
	{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\b`)},
	{"database_url", regexp.MustCompile(`\b(postgres|postgresql|mysql|mongodb|redis)(\+[a-z]+)?://[^:\s]+:[^@\s]+@[^\s'"]+`)},
	{"generic_api_key", regexp.MustCompile(`(?i)\b(api[_-]?key|apikey|secret|token|password|passwd)\s*[:=]\s*['"][A-Za-z0-9_\-]{16,}['"]`)},
}

// Scan walks files (or paths) and returns secret findings.
func Scan(paths []string, cfg *config.SecretsCfg, projectRoot string) ([]Finding, error) {
	var out []Finding
	max := cfg.MaxFileBytes
	if max <= 0 {
		max = 1024 * 1024
	}
	for _, p := range paths {
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			filepath.Walk(p, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				if ignored(path, cfg.IgnorePaths, projectRoot) {
					return nil
				}
				if info.Size() > max {
					return nil
				}
				out = append(out, scanFile(path)...)
				return nil
			})
			continue
		}
		if ignored(p, cfg.IgnorePaths, projectRoot) || fi.Size() > max {
			continue
		}
		out = append(out, scanFile(p)...)
	}
	return out, nil
}

func ignored(path string, patterns []string, root string) bool {
	return filesystem.MatchAny(path, patterns, root)
}

func scanFile(path string) []Finding {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Finding
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if isBinary(line) {
			return nil
		}
		for _, r := range rules {
			if m := r.re.FindString(line); m != "" {
				out = append(out, Finding{File: path, Line: lineNo, Type: r.name, Redact: Redact(m)})
			}
		}
	}
	return out
}

func isBinary(s string) bool {
	for i := 0; i < len(s) && i < 256; i++ {
		if s[i] == 0 {
			return true
		}
	}
	return false
}

// Redact keeps first 3 and last 2 chars, masks rest.
func Redact(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:3] + strings.Repeat("*", len(s)-5) + s[len(s)-2:]
}
