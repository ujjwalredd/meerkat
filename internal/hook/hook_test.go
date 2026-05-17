package hook

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "bash allow",
			in:   `{"tool_name":"Bash","tool_input":{"command":"npm test"}}`,
			want: "allow",
		},
		{
			name: "bash block",
			in:   `{"tool_name":"Bash","tool_input":{"command":"sudo rm -rf /"}}`,
			want: "deny",
		},
		{
			name: "bash ask",
			in:   `{"tool_name":"Bash","tool_input":{"command":"npm install left-pad"}}`,
			want: "ask",
		},
		{
			name: "write allowed",
			in: fmt.Sprintf(
				`{"tool_name":"Write","tool_input":{"file_path":%q}}`,
				filepath.Join(d, "src", "x.go"),
			),
			want: "allow",
		},
		{
			name: "write outside project",
			in: fmt.Sprintf(
				`{"tool_name":"Write","tool_input":{"file_path":%q}}`,
				filepath.Join(d, "..", "outside.go"),
			),
			want: "deny",
		},
		{
			name: "read blocked env",
			in: fmt.Sprintf(
				`{"tool_name":"Read","tool_input":{"file_path":%q}}`,
				filepath.Join(d, ".env"),
			),
			want: "deny",
		},
		{
			name: "multiedit allowed",
			in: fmt.Sprintf(
				`{"tool_name":"MultiEdit","tool_input":{"file_path":%q}}`,
				filepath.Join(d, "src", "x.go"),
			),
			want: "allow",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := PreToolUse(strings.NewReader(tc.in), &out, p); err != nil {
				t.Fatal(err)
			}
			if got := permissionDecision(t, out.Bytes()); got != tc.want {
				t.Fatalf("want %s got %s; output=%s", tc.want, got, out.String())
			}
		})
	}
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
