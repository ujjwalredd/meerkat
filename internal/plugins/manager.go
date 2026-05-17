// Package plugins manages out-of-process plugins that contribute evidence
// to the decision engine. Plugins are exec-based: meerkat invokes the
// binary (e.g. `gitleaks`, `trufflehog`) and parses JSON output.
//
// Plugins NEVER change the decision. They only contribute findings.
// The decision engine remains the single source of ALLOW/ASK/BLOCK.
package plugins

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/ujjwalredd/meerkat/internal/scanner"
)

// Scanner is the secret-scanner plugin contract.
type Scanner interface {
	Name() string
	Available() bool
	Scan(ctx context.Context, paths []string) ([]scanner.Finding, error)
}

// EnabledScanners returns the scanner plugins requested by the policy
// whose binaries are present on PATH.
func EnabledScanners(want []string) []Scanner {
	all := []Scanner{Gitleaks{}, Trufflehog{}}
	wanted := map[string]bool{}
	for _, w := range want {
		wanted[w] = true
	}
	var out []Scanner
	for _, s := range all {
		if wanted[s.Name()] && s.Available() {
			out = append(out, s)
		}
	}
	return out
}

// HasBinary is a helper for plugin Available() impls.
func HasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// MergeFindings appends plugin findings to core findings, dedup by file+line+type.
func MergeFindings(core []scanner.Finding, extras ...[]scanner.Finding) []scanner.Finding {
	seen := map[string]bool{}
	key := func(f scanner.Finding) string { return fmt.Sprintf("%s:%d:%s", f.File, f.Line, f.Type) }
	out := make([]scanner.Finding, 0, len(core))
	for _, f := range core {
		if !seen[key(f)] {
			seen[key(f)] = true
			out = append(out, f)
		}
	}
	for _, es := range extras {
		for _, f := range es {
			if !seen[key(f)] {
				seen[key(f)] = true
				out = append(out, f)
			}
		}
	}
	return out
}
