package hook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestPreToolUseAllow(t *testing.T) {
	p := config.Default()
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"npm test"}}`)
	var out bytes.Buffer
	if err := PreToolUse(in, &out, p); err != nil {
		t.Fatal(err)
	}
	var r struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &r); err != nil {
		t.Fatalf("invalid output: %v\n%s", err, out.String())
	}
	if r.HookSpecificOutput.PermissionDecision != "allow" {
		t.Errorf("want allow got %q", r.HookSpecificOutput.PermissionDecision)
	}
}

func TestPreToolUseJSONFixtures(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := config.Default()
	p.Project.Root = d
	p.FS.AllowedWritePaths = []string{"./src"}
	p.FS.BlockedPaths = []string{"./.env"}
	outsidePath := filepath.Join(d, "..", "outside.go")

	cases := []struct {
		name    string
		fixture string
		want    string
	}{
		{
			name:    "bash allow",
			fixture: "pretooluse_bash_allow.json",
			want:    "allow",
		},
		{
			name:    "bash block",
			fixture: "pretooluse_bash_block.json",
			want:    "deny",
		},
		{
			name:    "bash ask",
			fixture: "pretooluse_bash_ask.json",
			want:    "ask",
		},
		{
			name:    "write allowed",
			fixture: "pretooluse_write_allowed.json",
			want:    "allow",
		},
		{
			name:    "write outside project",
			fixture: "pretooluse_write_outside_project.json",
			want:    "deny",
		},
		{
			name:    "read blocked env",
			fixture: "pretooluse_read_blocked.json",
			want:    "deny",
		},
		{
			name:    "multiedit allowed",
			fixture: "pretooluse_multiedit_allowed.json",
			want:    "allow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := loadHookFixture(t, tc.fixture, d, outsidePath)
			var out bytes.Buffer
			if err := PreToolUse(strings.NewReader(in), &out, p); err != nil {
				t.Fatal(err)
			}
			if got := permissionDecision(t, out.Bytes()); got != tc.want {
				t.Fatalf("want %s got %s; output=%s", tc.want, got, out.String())
			}
		})
	}
}

func loadHookFixture(t *testing.T, name, root, outsidePath string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(b), "__ROOT__", filepath.ToSlash(root))
	s = strings.ReplaceAll(s, "__OUTSIDE_PATH__", filepath.ToSlash(outsidePath))
	return s
}

func TestPreToolUseBlock(t *testing.T) {
	p := config.Default()
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"sudo rm -rf /"}}`)
	var out bytes.Buffer
	if err := PreToolUse(in, &out, p); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
		t.Errorf("want deny, got %s", out.String())
	}
}

func TestPreToolUseBlockProtectedPush(t *testing.T) {
	p := config.Default()
	in := strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}`)
	var out bytes.Buffer
	_ = PreToolUse(in, &out, p)
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) {
		t.Errorf("protected push must deny: %s", out.String())
	}
}

func TestPreToolUsePassNonBash(t *testing.T) {
	p := config.Default()
	in := strings.NewReader(`{"tool_name":"Read","tool_input":{"file_path":"x"}}`)
	var out bytes.Buffer
	_ = PreToolUse(in, &out, p)
	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("non-Bash must pass-through: %s", out.String())
	}
}

func permissionDecision(t *testing.T, b []byte) string {
	t.Helper()
	var r struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(b), &r); err != nil {
		t.Fatalf("invalid hook output: %v\n%s", err, string(b))
	}
	return r.HookSpecificOutput.PermissionDecision
}
