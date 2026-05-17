// Package plugins manages out-of-process plugins that contribute evidence
// to the decision engine. Plugins are exec-based for v0.3: meerkat spawns
// the binary, feeds JSON on stdin, reads JSON on stdout. gRPC over UDS is
// planned for v0.4 once a stable plugin SDK ships.
//
// Plugins NEVER change the decision. They can only raise risk or contribute
// findings. The decision engine remains the single source of truth.
package plugins

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/ujjwalredd/meerkat/internal/audit"
	"github.com/ujjwalredd/meerkat/internal/scanner"
)

// Scanner is the secret-scanner plugin contract.
type Scanner interface {
	Name() string
	Available() bool
	Scan(ctx context.Context, paths []string) ([]scanner.Finding, error)
}

// Classifier is the command-classifier plugin contract.
type Classifier interface {
	Name() string
	Available() bool
	// Classify returns extra risk level ("low"|"medium"|"high") and
	// reasons. Core only RAISES risk; never lowers.
	Classify(ctx context.Context, cmdline string) (risk string, reasons []string, err error)
}

// AuditSink is an additional destination for audit events.
type AuditSink interface {
	Name() string
	Available() bool
	Emit(e audit.Event) error
}

// Registry holds loaded plugins.
type Registry struct {
	mu          sync.RWMutex
	scanners    []Scanner
	classifiers []Classifier
	sinks       []AuditSink
}

func NewRegistry() *Registry { return &Registry{} }

func (r *Registry) RegisterScanner(s Scanner) {
	r.mu.Lock()
	r.scanners = append(r.scanners, s)
	r.mu.Unlock()
}
func (r *Registry) RegisterClassifier(c Classifier) {
	r.mu.Lock()
	r.classifiers = append(r.classifiers, c)
	r.mu.Unlock()
}
func (r *Registry) RegisterSink(s AuditSink) {
	r.mu.Lock()
	r.sinks = append(r.sinks, s)
	r.mu.Unlock()
}

func (r *Registry) Scanners() []Scanner {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Scanner(nil), r.scanners...)
}
func (r *Registry) Classifiers() []Classifier {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Classifier(nil), r.classifiers...)
}
func (r *Registry) Sinks() []AuditSink {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Sink(nil), nil)[:0]
}

// HasBinary is a helper for plugin Available() impls.
func HasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// MergeFindings appends plugin findings to core findings, dedup by file+line+type.
func MergeFindings(core []scanner.Finding, extras [][]scanner.Finding) []scanner.Finding {
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

// type Sink alias kept for forward compat
type Sink = AuditSink
