//go:build windows

// Package wsl2 re-execs the command inside the user's default WSL2 distro
// so Windows workflows can use the full Linux backend stack.
package wsl2

import (
	"os/exec"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
)

func init() { sandbox.Register(&Backend{}) }

type Backend struct{}

func (Backend) Name() string { return "wsl2" }

func (Backend) Available() bool {
	_, err := exec.LookPath("wsl.exe")
	return err == nil
}

func (Backend) Wrap(argv []string, p *config.Policy) ([]string, sandbox.Cleanup, error) {
	// Re-exec the original argv inside WSL with the project dir bind-mounted
	// (WSL auto-mounts /mnt/c by default). Future versions also re-invoke
	// `meerkat run --sandbox=bwrap` inside the distro for layered isolation.
	wrapped := append([]string{"wsl.exe", "--cd", p.Project.Root, "--"}, argv...)
	return wrapped, nil, nil
}
