// Package hook implements Claude Code hook event handlers.
//
// Wire-up: ~/.claude/settings.json runs `meerkat hook <event>` per event.
// Each handler reads a JSON event from stdin, writes a JSON response to
// stdout per Claude Code's hook protocol. Exit code 0 unless the binary
// itself failed; permission decisions go in the JSON payload.
package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/ujjwalredd/meerkat/internal/config"
	"github.com/ujjwalredd/meerkat/internal/decision"
)

// hookInput is the subset of Claude Code's hook JSON we care about.
type hookInput struct {
	SessionID      string         `json:"session_id"`
	TranscriptPath string         `json:"transcript_path"`
	ToolName       string         `json:"tool_name"`
	ToolInput      map[string]any `json:"tool_input"`
}

// preToolUseOut uses the new hookSpecificOutput shape (Claude Code 2024+).
type preToolUseOut struct {
	HookSpecificOutput map[string]any `json:"hookSpecificOutput"`
}

// PreToolUse classifies a Bash tool call. ALLOW → allow, BLOCK → deny,
// ASK → ask (Claude Code shows its own prompt). Non-Bash tools pass.
func PreToolUse(in io.Reader, out io.Writer, p *config.Policy) error {
	var ev hookInput
	if err := json.NewDecoder(in).Decode(&ev); err != nil {
		return err
	}
	if ev.ToolName != "Bash" {
		return writeJSON(out, map[string]any{}) // pass-through
	}
	cmd, _ := ev.ToolInput["command"].(string)
	if cmd == "" {
		return writeJSON(out, map[string]any{})
	}
	d, _ := decision.Decide(cmd, p)
	perm := "ask"
	switch d.Action {
	case decision.Allow:
		perm = "allow"
	case decision.Block:
		perm = "deny"
	}
	reason := joinReasons(d.Reasons)
	return writeJSON(out, preToolUseOut{
		HookSpecificOutput: map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       perm,
			"permissionDecisionReason": fmt.Sprintf("meerkat: %s (%s) — %s", d.Action, d.RiskLevel, reason),
		},
	})
}

// SessionStart spawns caffeinate (macOS) / systemd-inhibit (Linux) detached,
// writes the PID to ~/.meerkat/keeper.pid so Stop can clean it up.
func SessionStart(_ io.Reader, out io.Writer) error {
	pidPath := keeperPIDPath()
	// If a keeper is already alive, do nothing.
	if pid := readPID(pidPath); pid > 0 && alive(pid) {
		return writeJSON(out, map[string]any{
			"additionalContext": fmt.Sprintf("[meerkat] keep-awake already active (pid %d)", pid),
		})
	}
	bin, args := keeperCmd()
	if bin == "" {
		return writeJSON(out, map[string]any{
			"additionalContext": "[meerkat] keep-awake unavailable on " + runtime.GOOS,
		})
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	// Detach: new session so the child outlives this hook process.
	cmd.SysProcAttr = detachAttr()
	if err := cmd.Start(); err != nil {
		return writeJSON(out, map[string]any{
			"additionalContext": "[meerkat] keep-awake failed: " + err.Error(),
		})
	}
	pid := cmd.Process.Pid
	_ = os.MkdirAll(filepath.Dir(pidPath), 0o700)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o600)
	// Release the child so this process can exit immediately. After this
	// cmd.Process.Pid becomes -1, so we use the captured pid below.
	_ = cmd.Process.Release()
	return writeJSON(out, map[string]any{
		"additionalContext": fmt.Sprintf("[meerkat] keep-awake active via %s (pid %d)", bin, pid),
	})
}

// Stop kills the keep-awake child if any.
func Stop(_ io.Reader, out io.Writer) error {
	pidPath := keeperPIDPath()
	pid := readPID(pidPath)
	if pid > 0 && alive(pid) {
		killPID(pid)
	}
	_ = os.Remove(pidPath)
	return writeJSON(out, map[string]any{})
}

func writeJSON(w io.Writer, v any) error {
	return json.NewEncoder(w).Encode(v)
}

func joinReasons(rs []string) string {
	if len(rs) == 0 {
		return "no reasons"
	}
	s := rs[0]
	for i := 1; i < len(rs); i++ {
		s += "; " + rs[i]
	}
	return s
}

func keeperPIDPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".meerkat", "keeper.pid")
}

func keeperCmd() (string, []string) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("caffeinate"); err == nil {
			// -i prevent idle sleep; -m disk; -s system; -d display. No -t = until killed.
			return "caffeinate", []string{"-imsd"}
		}
	case "linux":
		if _, err := exec.LookPath("systemd-inhibit"); err == nil {
			return "systemd-inhibit", []string{
				"--what=idle:sleep", "--who=meerkat",
				"--why=meerkat session active", "--mode=block",
				"sleep", "86400",
			}
		}
	}
	return "", nil
}

func readPID(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(string(b))
	return pid
}
