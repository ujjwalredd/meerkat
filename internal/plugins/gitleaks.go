package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/ujjwalredd/meerkat/internal/scanner"
)

type Gitleaks struct{}

func (Gitleaks) Name() string    { return "gitleaks" }
func (Gitleaks) Available() bool { return HasBinary("gitleaks") }

// gitleaksResult is the subset of gitleaks JSON we consume.
type gitleaksResult struct {
	File        string `json:"File"`
	StartLine   int    `json:"StartLine"`
	RuleID      string `json:"RuleID"`
	Description string `json:"Description"`
	Secret      string `json:"Secret"`
}

func (Gitleaks) Scan(ctx context.Context, paths []string) ([]scanner.Finding, error) {
	if !HasBinary("gitleaks") {
		return nil, fmt.Errorf("gitleaks: not installed")
	}
	out := []scanner.Finding{}
	for _, p := range paths {
		var buf bytes.Buffer
		cmd := exec.CommandContext(ctx,
			"gitleaks", "detect",
			"--no-banner", "--no-git", "--redact",
			"--report-format=json", "--report-path=/dev/stdout",
			"--source="+p)
		cmd.Stdout = &buf
		_ = cmd.Run() // gitleaks exits non-zero on findings; ignore exit
		var rs []gitleaksResult
		if err := json.Unmarshal(buf.Bytes(), &rs); err != nil {
			continue
		}
		for _, r := range rs {
			out = append(out, scanner.Finding{
				File: r.File, Line: r.StartLine,
				Type:   "gitleaks:" + r.RuleID,
				Redact: scanner.Redact(r.Secret),
			})
		}
	}
	return out, nil
}
