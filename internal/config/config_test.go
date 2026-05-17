package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultValid(t *testing.T) {
	p := Default()
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveLoad(t *testing.T) {
	d := t.TempDir()
	p := Default()
	path := filepath.Join(d, "meerkat.yml")
	if err := Save(p, path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.Mode.DefaultAction != "ask" {
		t.Errorf("bad round-trip: %+v", got)
	}
}

func TestMissingPolicy(t *testing.T) {
	_, err := Load("/nonexistent/meerkat.yml")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestInvalidVersion(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x.yml")
	os.WriteFile(p, []byte("version: 99\nproject:\n  root: .\n"), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("want version error")
	}
}

func TestInvalidDefaultActionWithOutsideProjectDisabled(t *testing.T) {
	p := Default()
	p.FS.BlockOutsideProject = false
	p.Mode.DefaultAction = "maybe"
	if err := p.Validate(); err == nil {
		t.Fatal("want mode.default_action error")
	}
}

func TestAllowedWritePathStartingWithDotsInsideRoot(t *testing.T) {
	d := t.TempDir()
	p := Default()
	p.Project.Root = d
	p.FS.AllowedWritePaths = []string{"./..cache"}
	if err := p.Validate(); err != nil {
		t.Fatalf("path is inside root and should validate: %v", err)
	}
}
