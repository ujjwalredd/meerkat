// Package appcontainer is the Windows AppContainer backend (beta stub).
// Full impl requires CreateAppContainerProfile + SetAppContainerInformation
// via golang.org/x/sys/windows. Gated by `meerkat_appcontainer` build tag.
package appcontainer

import (
	"fmt"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/sandbox"
)

func init() { sandbox.Register(&Backend{}) }

type Backend struct{}

func (Backend) Name() string    { return "appcontainer" }
func (Backend) Available() bool { return false }

func (Backend) Wrap(argv []string, p *config.Policy) ([]string, sandbox.Cleanup, error) {
	return nil, nil, fmt.Errorf("appcontainer: backend not enabled in this build (beta; see docs/v1-architecture.md)")
}
