package sandbox

import (
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

type stubAvail struct {
	name string
	ok   bool
}

func (s stubAvail) Name() string    { return s.name }
func (s stubAvail) Available() bool { return s.ok }
func (s stubAvail) Wrap(argv []string, _ *config.Policy) ([]string, Cleanup, error) {
	return argv, nil, nil
}

func TestSelectOffAndAuto(t *testing.T) {
	if b, err := Select("off", true); err != nil || b != nil {
		t.Errorf("off should return nil,nil; got %v,%v", b, err)
	}
	if b, err := Select("", true); err != nil || b != nil {
		t.Errorf("empty should return nil,nil; got %v,%v", b, err)
	}
}

func TestSelectUnknown(t *testing.T) {
	if _, err := Select("nonexistent-backend", false); err == nil {
		t.Error("unknown backend should error")
	}
}

func TestSelectFailClosedOnUnavailable(t *testing.T) {
	Register(stubAvail{name: "test-unavail", ok: false})
	if _, err := Select("test-unavail", true); err == nil {
		t.Error("fail_closed + unavailable should error")
	}
	if b, err := Select("test-unavail", false); err != nil || b != nil {
		t.Errorf("non-fail-closed unavailable should return nil,nil; got %v,%v", b, err)
	}
}
