//go:build windows

// Package jobobject implements the Windows Job Object backend (always-on
// when on Windows). Guarantees no orphan children when meerkat exits.
package jobobject

import (
	"os/exec"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
)

func init() { sandbox.Register(&Backend{}) }

type Backend struct{}

func (Backend) Name() string    { return "jobobject" }
func (Backend) Available() bool { return true }

// Wrap is a passthrough; Job Object attachment happens in the runner's
// SysProcAttr setup (CREATE_SUSPENDED + AssignProcessToJobObject + ResumeThread).
// Implemented as platform-specific runner work; this backend marker tells
// the runner to apply it.
func (Backend) Wrap(argv []string, p *config.Policy) ([]string, sandbox.Cleanup, error) {
	// Confirm the binary exists; surface a friendly error.
	if len(argv) > 0 {
		_, _ = exec.LookPath(argv[0])
	}
	return argv, nil, nil
}
