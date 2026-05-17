# Use Meerkat inside Claude Code

Goal: open Claude Code, type **`/meerkat <prompt>`**, and have Meerkat
already enforcing the policy in the background — keep-awake on, safe
shell commands auto-approved, dangerous commands blocked, no extra
wrapper invocation.

## One-time setup

```bash
# 1. install meerkat (any method)
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | bash
#  -or- brew/go/npx

# 2. install the /meerkat slash command + Claude Code hooks
meerkat claude install

# 3. (optional) wire the MCP server too
claude mcp add meerkat -- meerkat mcp start

# 4. in each project you use
cd ~/my-project
meerkat init --profile=agent     # or basic|strict|node|python
```

Or use the explicit one-command installer path:

```bash
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | MEERKAT_SETUP_CLAUDE=1 bash
cd ~/my-project
meerkat init --profile=agent
```

That's it. From now on, **every** Claude Code session you start runs under
Meerkat's hooks automatically. You do not have to wrap `claude` in
`meerkat run`. You do not have to remember anything.

## What `meerkat claude install` does

1. Writes `~/.claude/commands/meerkat.md` — a slash command that takes
   your prompt and tells Claude to respect the policy.
2. Merges three hooks into `~/.claude/settings.json`:
   - **PreToolUse (Bash)** → `meerkat hook pretooluse`
     Every shell command Claude wants to run is classified. ALLOW →
     auto-approved (no prompt). BLOCK → refused with reason. ASK → falls
     through to Claude Code's normal approval flow.
   - **SessionStart** → `meerkat hook sessionstart`
     Spawns `caffeinate -imsd` (macOS) / `systemd-inhibit` (Linux)
     detached. Records PID in `~/.meerkat/keeper.pid`. Mac stays awake.
   - **Stop** → `meerkat hook stop`
     Reads the PID, kills the keeper, removes the file.

Existing hooks in `~/.claude/settings.json` are preserved.

## Daily use

```text
> /meerkat refactor the auth middleware and add tests
```

Claude works on the task. Behind the scenes:

- Laptop screen can close, machine doesn't sleep.
- `npm test`, `go test ./...`, `git status`, `git diff` → auto-approved.
- `npm install`, `git commit`, `git push` (non-protected) → Claude Code
  shows its prompt (Meerkat returned ASK).
- `sudo`, `curl`, `wget`, `rm -rf /`, `git push origin main`,
  `git push --force` → **denied silently** with reason logged.
- When you `/clear` or close the session, caffeinate dies.

## Verify it's wired

```bash
# 1. settings file references meerkat
grep -A3 PreToolUse ~/.claude/settings.json | grep meerkat

# 2. dry-run the hook
echo '{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}' \
  | meerkat hook pretooluse
# → {"hookSpecificOutput":{"permissionDecision":"deny", ...}}

# 3. sanity-check session lifecycle
echo "{}" | meerkat hook sessionstart   # starts caffeinate, prints PID
echo "{}" | meerkat hook stop           # kills it
```

## Uninstall

```bash
meerkat claude uninstall
```

Removes `~/.claude/commands/meerkat.md` and deletes only the three
Meerkat hooks. Your other settings/hooks are left alone.

## How auto-approval is decided

The hook reads `meerkat.yml` in the **current working directory** Claude
Code launched from. Fallbacks: `~/.meerkat/meerkat.yml`, then built-in
defaults. So per-project policies just work — commit `meerkat.yml` to
the repo and the whole team gets the same auto-approval behavior.

Customize the auto-approve list per project: see
[`auto-approval.md`](auto-approval.md).

## Limitations

- Hooks only see what Claude Code's tool layer exposes. Inner subshells
  spawned by an allowed command (`npm test` runs `node` which runs
  whatever) are **not** intercepted. Shell-proxy mode is planned for v0.5.
- The hook's view of the policy is whatever `meerkat.yml` is on disk at
  the moment the hook fires. Editing the policy mid-session takes effect
  on the next tool call, not the running one.
- macOS `caffeinate` and Linux `systemd-inhibit` must be in PATH; on
  Windows the SessionStart hook is a no-op and prints that fact via
  `additionalContext`.
- If `meerkat` is not in the PATH Claude Code sees (rare; happens when
  installed under a Node version manager), edit `~/.claude/settings.json`
  to use the absolute path.
