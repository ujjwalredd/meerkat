// Package landlock is the Linux Landlock LSM backend (beta stub).
//
// Full Landlock self-restriction requires the landlock_create_ruleset /
// landlock_add_rule / landlock_restrict_self syscalls (kernel >= 5.13).
// Shipping the native syscall path is gated behind a dedicated build tag
// because it needs a recent kernel to test against. The default build
// registers the backend in unavailable mode so `sandbox.Auto()` will skip
// it and pick `bwrap` on Linux.
//
// To enable the real impl, add a build tag `meerkat_landlock` and provide
// a syscall wrapper (planned via golang.org/x/sys/unix once Landlock v3
// helpers land upstream).
package landlock

import (
	"fmt"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
)

func init() { sandbox.Register(&Backend{}) }

type Backend struct{}

func (Backend) Name() string    { return "landlock" }
func (Backend) Available() bool { return false } // see package doc; enable via build tag

func (Backend) Wrap(argv []string, p *config.Policy) ([]string, sandbox.Cleanup, error) {
	return nil, nil, fmt.Errorf("landlock: backend not enabled in this build (beta; see docs/v1-architecture.md)")
}
