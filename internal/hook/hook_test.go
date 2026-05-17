package hook

import (
	"bytes"
	"encoding/json"
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
