package main

import (
	"path/filepath"
	"testing"

	"github.com/ujjwalredd/meerkat/internal/config"
)

func TestInstallClaudeHookPreservesExistingAndIsIdempotent(t *testing.T) {
	hooks := map[string]any{
		"PreToolUse": []any{map[string]any{
			"matcher": "Bash",
			"hooks": []any{map[string]any{
				"type":    "command",
				"command": "echo keep",
				"timeout": 1,
			}},
		}},
	}
	spec := claudeHookSpec{
		EventKey: "PreToolUse",
		HookArg:  "pretooluse",
		Matcher:  "Bash|Write|Edit|MultiEdit|NotebookEdit|Read",
		Command:  `"/usr/local/bin/meerkat" hook pretooluse`,
	}

	installClaudeHook(hooks, spec)
	installClaudeHook(hooks, spec)

	entries := hookEntries(hooks["PreToolUse"])
	if len(entries) != 2 {
		t.Fatalf("want existing hook + one meerkat hook, got %d: %#v", len(entries), entries)
	}
	if !hasCommand(entries, "echo keep") {
		t.Fatalf("existing hook was not preserved: %#v", entries)
	}
	meerkat := 0
	for _, entry := range entries {
		if isMeerkatHookEntry(entry, "pretooluse") {
			meerkat++
		}
	}
	if meerkat != 1 {
		t.Fatalf("want one meerkat hook after reinstall, got %d: %#v", meerkat, entries)
	}
}

func TestRemoveClaudeHookOnlyRemovesMeerkat(t *testing.T) {
	hooks := map[string]any{}
	installClaudeHook(hooks, claudeHookSpec{
		EventKey: "Stop",
		HookArg:  "stop",
		Command:  `"/usr/local/bin/meerkat" hook stop`,
	})
	hooks["Stop"] = append(hookEntries(hooks["Stop"]), map[string]any{
		"hooks": []any{map[string]any{
			"type":    "command",
			"command": "echo keep",
			"timeout": 1,
		}},
	})

	removeClaudeHook(hooks, "Stop", "stop")

	entries := hookEntries(hooks["Stop"])
	if len(entries) != 1 || !hasCommand(entries, "echo keep") {
		t.Fatalf("uninstall should preserve non-meerkat hooks, got %#v", entries)
	}
}

func TestOutOfScopeUsesResolvedAllowedPaths(t *testing.T) {
	d := t.TempDir()
	p := config.Default()
	p.Project.Root = d
	p.FS.AllowedWritePaths = []string{"./src"}

	got := outOfScope([]string{
		filepath.Join(d, "src", "x.go"),
		filepath.Join(d, "README.md"),
	}, p)
	if len(got) != 1 || got[0] != filepath.Join(d, "README.md") {
		t.Fatalf("want only README out of scope, got %#v", got)
	}
}

func hasCommand(entries []any, want string) bool {
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		hooks, ok := m["hooks"].([]any)
		if !ok {
			continue
		}
		for _, h := range hooks {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			if cmd, _ := hm["command"].(string); cmd == want {
				return true
			}
		}
	}
	return false
}
