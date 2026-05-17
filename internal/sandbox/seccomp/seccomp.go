// Package seccomp is the Linux seccomp-bpf backend (beta stub).
//
// Production seccomp profiles need libseccomp-golang or a hand-rolled BPF
// program. To keep the binary cgo-free and statically linked, the real
// impl is gated behind a `meerkat_seccomp` build tag. The default build
// registers the backend in unavailable mode.
package seccomp

import (
	"fmt"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
)

func init() { sandbox.Register(&Backend{}) }

type Backend struct{}

func (Backend) Name() string    { return "seccomp" }
func (Backend) Available() bool { return false }

func (Backend) Wrap(argv []string, p *config.Policy) ([]string, sandbox.Cleanup, error) {
	return nil, nil, fmt.Errorf("seccomp: backend not enabled in this build (beta; see docs/v1-architecture.md)")
}
