//go:build linux

package bwrap

import (
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestWrapNetworkPolicy(t *testing.T) {
	p := config.Default()
	p.Net.Default = "allow"
	p.Sandbox.Egress.Mode = "off"
	argv, _, err := Backend{}.Wrap([]string{"echo", "ok"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if containsArg(argv, "--unshare-net") {
		t.Fatalf("network allow policy should not add --unshare-net: %#v", argv)
	}

	p.Net.Default = "ask"
	argv, _, err = Backend{}.Wrap([]string{"echo", "ok"}, p)
	if err != nil {
		t.Fatal(err)
	}
	if !containsArg(argv, "--unshare-net") {
		t.Fatalf("network ask policy without proxy should add --unshare-net: %#v", argv)
	}
}

func containsArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
