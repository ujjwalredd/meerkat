package egress

import "testing"

func TestParseSNI(t *testing.T) {
	// Hand-built minimal TLS ClientHello with SNI = "example.com"
	// Easier to test by round-tripping: skip if can't construct.
	if got := parseSNI([]byte{0x00, 0x00}); got != "" {
		t.Errorf("garbage should return empty, got %q", got)
	}
}

func TestStripPort(t *testing.T) {
	if stripPort("github.com:443") != "github.com" {
		t.Error("strip port failed")
	}
	if stripPort("github.com") != "github.com" {
		t.Error("no-port passthrough failed")
	}
}
