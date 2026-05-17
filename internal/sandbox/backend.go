// Package sandbox defines the enforcement backend interface and an Auto
// selector. Backends translate a policy into a wrapped exec.Cmd or apply
// in-process restrictions to the child. The decision engine is unchanged;
// sandbox only enforces what the decision already allowed.
package sandbox

import (
	"fmt"
	"runtime"

	"github.com/ujjwalredd/meerkat/internal/config"
)

// Backend wraps a command argv into a sandboxed argv.
// Returns the wrapped argv and a Cleanup func (may be nil).
type Backend interface {
	Name() string
	Available() bool
	Wrap(argv []string, p *config.Policy) ([]string, Cleanup, error)
}

type Cleanup func()

var registry = map[string]Backend{}

// Register a backend at init time.
func Register(b Backend) { registry[b.Name()] = b }

// Get returns a backend by name.
func Get(name string) (Backend, bool) {
	b, ok := registry[name]
	return b, ok
}

// List returns all registered backend names.
func List() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	return out
}

// Auto picks the best available backend for the current OS. Returns nil
// if none available. Caller decides whether to fail closed.
func Auto() Backend {
	var prefs []string
	switch runtime.GOOS {
	case "darwin":
		prefs = []string{"seatbelt"}
	case "linux":
		prefs = []string{"bwrap"}
	case "windows":
		prefs = []string{"wsl2", "jobobject"}
	}
	for _, n := range prefs {
		if b, ok := registry[n]; ok && b.Available() {
			return b
		}
	}
	return nil
}

// Select returns the backend named by --sandbox=<n> / sandbox.backend.
// "auto" → Auto(). "off" / "" → nil, nil. Unknown → error.
// If fail_closed and backend missing → error.
func Select(name string, failClosed bool) (Backend, error) {
	switch name {
	case "", "off":
		return nil, nil
	case "auto":
		b := Auto()
		if b == nil && failClosed {
			return nil, fmt.Errorf("sandbox: no backend available on %s and fail_closed=true", runtime.GOOS)
		}
		return b, nil
	}
	b, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("sandbox: unknown backend %q (available: %v)", name, List())
	}
	if !b.Available() {
		if failClosed {
			return nil, fmt.Errorf("sandbox: backend %q not available on this host and fail_closed=true", name)
		}
		return nil, nil
	}
	return b, nil
}
