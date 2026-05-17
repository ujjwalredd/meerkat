//go:build linux

// Package bwrap implements the Linux bubblewrap backend. Uses user
// namespaces with no setuid binary required (kernel.unprivileged_userns_clone=1).
// Pair with the egress proxy + --unshare-net for network enforcement.
package bwrap

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/filesystem"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
)

func init() { sandbox.Register(&Backend{}) }

type Backend struct{}

func (Backend) Name() string { return "bwrap" }

func (Backend) Available() bool {
	_, err := exec.LookPath("bwrap")
	return err == nil
}

func (Backend) Wrap(argv []string, p *config.Policy) ([]string, sandbox.Cleanup, error) {
	if len(argv) == 0 {
		return nil, nil, fmt.Errorf("bwrap: empty argv")
	}
	root, _ := filepath.Abs(p.Project.Root)
	args := []string{
		"--die-with-parent",
		"--new-session",
		"--ro-bind", "/usr", "/usr",
		"--ro-bind", "/bin", "/bin",
		"--ro-bind", "/sbin", "/sbin",
		"--ro-bind", "/lib", "/lib",
		"--ro-bind", "/lib64", "/lib64",
		"--ro-bind", "/etc", "/etc",
		"--dev", "/dev",
		"--proc", "/proc",
		"--tmpfs", "/tmp",
		"--bind", root, root,
		"--chdir", root,
	}
	// Mask blocked paths by binding /dev/null over them.
	for _, bp := range p.FS.BlockedPaths {
		ep := filesystem.ExpandTilde(bp)
		if !filepath.IsAbs(ep) {
			ep = filepath.Join(root, ep)
		}
		args = append(args, "--ro-bind-try", "/dev/null", ep)
	}
	// Network: block direct egress unless policy explicitly allows it. When
	// the proxy is enabled, keep network available so proxy-aware tools can
	// connect to it.
	if p.Sandbox.Egress.Mode == "proxy" {
		// keep net; the egress proxy enforces policy via HTTP(S)_PROXY env.
	} else if p.Net.Default != "allow" {
		args = append(args, "--unshare-net")
	}
	args = append(args, "--")
	args = append(args, argv...)
	return append([]string{"bwrap"}, args...), nil, nil
}
