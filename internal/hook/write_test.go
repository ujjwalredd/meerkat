package hook

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestPreWriteAllowsAllowedPath(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "src"), 0o755)
	p := config.Default()
	p.Project.Root = d
	p.FS.AllowedWritePaths = []string{filepath.Join(d, "src")}
	target := filepath.Join(d, "src", "x.go")
	in := bytes.NewReader([]byte(fmt.Sprintf(
		`{"tool_name":"Write","tool_input":{"file_path":%q}}`, target)))
	var out bytes.Buffer
	_ = PreToolUse(in, &out, p)
	if !strings.Contains(out.String(), `"permissionDecision":"allow"`) {
		t.Errorf("want allow, got %s", out.String())
	}
}

func TestPreWriteDeniesBlockedPath(t *testing.T) {
	d := t.TempDir()
	p := config.Default()
	p.Project.Root = d
	p.FS.BlockedPaths = []string{filepath.Join(d, ".env")}
	in := bytes.NewReader([]byte(fmt.Sprintf(
		`{"tool_name":"Write","tool_input":{"file_path":%q}}`, filepath.Join(d, ".env"))))
	var out bytes.Buffer
	_ = PreToolUse(in, &out, p)
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
		t.Errorf("want deny, got %s", out.String())
	}
}

func TestPreReadDeniesBlockedPath(t *testing.T) {
	d := t.TempDir()
	p := config.Default()
	p.Project.Root = d
	p.FS.BlockedPaths = []string{filepath.Join(d, ".env")}
	in := bytes.NewReader([]byte(fmt.Sprintf(
		`{"tool_name":"Read","tool_input":{"file_path":%q}}`, filepath.Join(d, ".env"))))
	var out bytes.Buffer
	_ = PreToolUse(in, &out, p)
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
		t.Errorf("want deny, got %s", out.String())
	}
}
