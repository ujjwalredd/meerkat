package filesystem

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInside(t *testing.T) {
	parent := "/tmp/proj"
	if !Inside(parent, "/tmp/proj/src/a.go") {
		t.Error("want inside")
	}
	if Inside(parent, "/tmp/other/a.go") {
		t.Error("want outside")
	}
	if !Inside(parent, parent) {
		t.Error("equal should be inside")
	}
}

func TestMatchAny(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, ".env"), []byte("x"), 0o600)
	patterns := []string{".env", "./secrets"}
	if !MatchAny(filepath.Join(d, ".env"), patterns, d) {
		t.Error("want match .env")
	}
}

func TestSymlinkResolution(t *testing.T) {
	d := t.TempDir()
	outside := t.TempDir()
	// macOS /tmp is symlinked to /private/tmp; resolve both ends.
	outsideReal, _ := Resolve(outside)
	link := filepath.Join(d, "lnk")
	if err := os.Symlink(outsideReal, link); err != nil {
		t.Skip("symlink unsupported")
	}
	real, err := Resolve(link)
	if err != nil {
		t.Fatal(err)
	}
	if !Inside(outsideReal, real) {
		t.Errorf("symlink target not resolved: %s vs %s", real, outsideReal)
	}
}
