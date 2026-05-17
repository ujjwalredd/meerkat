# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.0] - 2026-05-17

### Added
- `PreToolUse` hook now also classifies **Write / Edit / MultiEdit /
  NotebookEdit / Read** tool calls (previously only Bash). File writes
  inside `filesystem.allowed_write_paths` are auto-approved; writes to
  `filesystem.blocked_paths` or outside `project.root` (when
  `block_outside_project: true`) are denied. Reads of blocked paths are
  denied. Anything else falls through to Claude Code's normal prompt.
- `meerkat claude install` now wires the hook matcher to
  `Bash|Write|Edit|MultiEdit|NotebookEdit|Read` instead of just `Bash`.
- `scripts/install.sh` supports `MEERKAT_SETUP_CLAUDE=1` for an explicit
  one-command CLI + Claude Code hook setup.

### Fixed
- `filesystem.Resolve` walks up to the nearest existing ancestor when
  the target path doesn't exist yet (typical for Write tool creating
  new files). Fixes scope-check correctness on macOS where `/tmp` is a
  symlink to `/private/tmp` — previously a brand-new file under
  `allowed_write_paths` could be miss-classified as outside scope.
- Claude hook install/uninstall preserves unrelated user hooks and avoids
  duplicating Meerkat hooks on reinstall.
- `audit.redact_secrets` now redacts secret-like values in audit events before
  writing JSONL logs.
- `secrets.enabled` and `secrets.scan_patterns` are honored by the built-in
  scanner.
- Auto-approve rules no longer allow high-risk commands just because a broad
  auto-approve pattern matched.
- Out-of-scope writes now return a policy violation exit code when
  `mode.deny_out_of_scope` is enabled.

### Changed
- README, install docs, and npm docs now separate "install the CLI" from
  "enable Claude Code `/meerkat` hooks" and document what is and is not
  protected.
- Installer no longer requires `tar` for raw binary installs and prints clearer
  next steps after installation.

## [0.3.0] - 2026-05-17

### Added — Claude Code integration (`/meerkat <prompt>`)
- `meerkat claude install` — writes `~/.claude/commands/meerkat.md`
  slash command + merges PreToolUse / SessionStart / Stop hooks into
  `~/.claude/settings.json`. Existing hooks preserved.
- `meerkat claude uninstall` — clean removal.
- `meerkat hook pretooluse` — classifies Claude-issued Bash tool calls
  via the decision engine, returns Claude Code's `hookSpecificOutput`
  (`permissionDecision: allow|deny|ask` + reason). Non-Bash passes through.
- `meerkat hook sessionstart` — spawns detached `caffeinate` / `systemd-inhibit`;
  PID at `~/.meerkat/keeper.pid`.
- `meerkat hook stop` — kills the keeper.

### Added — Sandbox backends (opt-in via `--sandbox=` or `sandbox.enabled`)
- Backend interface + `Auto` selector (`internal/sandbox`).
- **macOS Seatbelt** via `sandbox-exec` with generated `.sb` profile.
- **Linux bubblewrap** with user-namespace isolation, optional `--unshare-net`.
- **Windows Job Object** (always-on on Windows for orphan cleanup).
- **WSL2** re-exec backend.
- `meerkat sandbox doctor` / `meerkat sandbox profile`.

### Added — Network enforcement
- **Egress proxy** (`internal/sandbox/egress`): HTTP CONNECT + plain HTTP,
  TLS SNI sniffing to verify SNI matches CONNECT host (defeats domain
  fronting). Per-run, bound to `127.0.0.1:<random>`.
- `network_egress` audit event.

### Added — Plugins (exec-based)
- Plugin manager (`internal/plugins`) with `gitleaks` and `trufflehog`
  adapters. Auto-activate when binaries are on PATH.
- `MergeFindings` dedup helper. Plugins can only **raise** risk; the
  decision engine remains the single source of ALLOW/ASK/BLOCK.

### Added — Integrations
- **GitHub branch-protection** lookup (`internal/integrations/github`)
  with 1h cache and read-only token via env var.
- **MCP server** (`meerkat mcp start`) — JSON-RPC 2.0 over stdio,
  methods: `meerkat.explain`, `meerkat.scan`, `meerkat.approve`.

### Added — Install + UX
- `scripts/install.sh` — POSIX one-line installer. Detects OS/arch,
  pulls prebuilt binary from GitHub Releases, falls back to `go install`.
  Gracefully handles the "no releases yet" case.
- `npm/` package `meerkat-cli` — Node wrapper for native Windows + all
  POSIX shells. Downloads the Go binary to `~/.meerkat/bin/` on first run.
- `meerkat init [wizard] [--profile=basic|strict|agent|node|python]` —
  interactive setup + tailored starters.
- `meerkat mcp start` — canonical alias matches `claude mcp add meerkat
  -- meerkat mcp start`.
- `[meerkat] running:` / `[meerkat] sandbox:` / `[meerkat] egress proxy:`
  status lines before child exec.
- New docs: [docs/install.md](docs/install.md),
  [docs/auto-approval.md](docs/auto-approval.md),
  [docs/claude-integration.md](docs/claude-integration.md),
  [docs/v1-architecture.md](docs/v1-architecture.md).
- `Makefile` for `build` / `test` / `race` / `release-local`.

### Changed
- Policy schema gained optional `sandbox`, `plugins`, `integrations`
  sections. Absent = v0.1 behavior.
- `examples/{node,python,agent}/meerkat.yml` rewritten as **tailored**
  policies (previously byte-identical copies of `basic`).
- README rewritten for v0.3 — emphasizes the `/meerkat` Claude Code
  flow as the primary path.

### Fixed
- `gitguard.CurrentBranch` falls back to `git symbolic-ref --short HEAD`
  on unborn HEAD (fresh repos).
- Cross-platform process runner split (`runner_unix.go` /
  `runner_windows.go`); Windows build clean.

### Removed
- Stub sandbox backends `landlock`, `seccomp`, `appcontainer` (always
  reported unavailable). Native impls are tracked in
  [`docs/v1-architecture.md`](docs/v1-architecture.md) for a future release.

## [0.1.0] - 2026-05-17

### Added
- `meerkat init` generates strict default `meerkat.yml`.
- `meerkat run` supervises a child process under a policy.
- `meerkat explain` shows ALLOW / ASK / BLOCK without executing.
- `meerkat scan` runs built-in regex secret scanner.
- `meerkat status`, `meerkat doctor`, `meerkat policy validate`.
- Deterministic rule-based command classifier + decision engine.
- Built-in secret scanner: AWS, GitHub, OpenAI, Anthropic, Stripe,
  Slack, JWT, database URLs, private keys, generic API keys.
- Git guard: protected-branch push block, force-push block,
  scan-before-commit, scan-before-push.
- Filesystem scope checks with symlink resolution.
- Keep-awake: macOS `caffeinate`, Linux `systemd-inhibit`.
- JSONL audit log with secret redaction.
- Dry-run mode. Approval prompt with timeout (default deny).

### Security
- Default policy denies privilege escalation, network egress tools,
  recursive force-removal, push to `main`/`master`/`production`.
