//go:build darwin

// Package seatbelt implements the macOS sandbox-exec (Seatbelt) backend.
// Generates a TinyScheme .sb profile from the policy and wraps the child
// command. sandbox-exec is deprecated by Apple but still functional on
// macOS 14/15.
package seatbelt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/filesystem"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
)

func init() { sandbox.Register(&Backend{}) }

type Backend struct{}

func (Backend) Name() string { return "seatbelt" }

func (Backend) Available() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

func (Backend) Wrap(argv []string, p *config.Policy) ([]string, sandbox.Cleanup, error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("seatbelt: empty argv")
	}
	profile, err := GenerateProfile(p)
	if err != nil {
		return nil, nil, fmt.Errorf("seatbelt: generate profile: %w", err)
	}
	f, err := os.CreateTemp("", "meerkat-*.sb")
	if err != nil {
		return nil, nil, fmt.Errorf("seatbelt: temp profile: %w", err)
	}
	if _, err := f.WriteString(profile); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, nil, fmt.Errorf("seatbelt: write profile: %w", err)
	}
	f.Close()
	wrapped := append([]string{"sandbox-exec", "-f", f.Name()}, argv...)
	cleanup := func() { os.Remove(f.Name()) }
	return wrapped, cleanup, nil
}

// GenerateProfile renders a .sb profile from policy. Exposed for
// `meerkat sandbox profile show`.
func GenerateProfile(p *config.Policy) (string, error) {
	var b strings.Builder
	w := func(s string) { b.WriteString(s); b.WriteString("\n") }

	w("(version 1)")
	// Allow-default + deny-specific. Pure deny-default profiles on modern
	// macOS are too fragile for arbitrary CLI tools (libsystem, dyld, and
	// xpc paths shift between OS versions). For strict deny-default, use
	// the Linux backend in a VM (Lima/OrbStack) per docs/v1-architecture.md.
	w("(allow default)")
	w("(debug deny)")

	root, _ := filepath.Abs(p.Project.Root)
	w("; --- deny secret/blocked paths ---")
	for _, bp := range p.FS.BlockedPaths {
		ap := absPath(filesystem.ExpandTilde(bp), root)
		w(fmt.Sprintf("(deny file-read* (subpath %q))", ap))
		w(fmt.Sprintf("(deny file-write* (subpath %q))", ap))
	}
	w("; --- writes outside allowed write paths inside the project ---")
	w(fmt.Sprintf("(deny file-write* (subpath %q))", filepath.Join(root, ".git", "hooks")))

	w("; --- network ---")
	// Seatbelt only does coarse network rules; domain enforcement is the
	// egress proxy's job (internal/sandbox/egress).
	if p.Net.Default == "block" {
		w("(deny network-outbound (remote ip))")
		w("(allow network-outbound (remote unix-socket))")
	}
	return b.String(), nil
}

func absPath(p, root string) string {
	p = filesystem.ExpandTilde(p)
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	return filepath.Clean(p)
}
