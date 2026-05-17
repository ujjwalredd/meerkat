# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added — Install + UX
- `scripts/install.sh` — POSIX one-line installer. Detects OS/arch, pulls
  prebuilt binary from GitHub Releases, falls back to `go install`.
- `npm/` package `meerkat-cli` — Node wrapper. `npx meerkat-cli@latest …`
  works on all platforms incl. native Windows PowerShell/cmd. Downloads
  the Go binary to `~/.meerkat/bin/` on first run.
- `meerkat init wizard` — interactive setup (project name, profile, agent
  auto-approval, sandbox/egress opt-in).
- `meerkat init --profile=basic|strict|agent|node|python` — tailored starters.
- `meerkat mcp start` — canonical alias for `meerkat mcp`, matches
  `claude mcp add meerkat -- meerkat mcp start`.
- [docs/install.md](docs/install.md) — three install paths documented.
- [docs/auto-approval.md](docs/auto-approval.md) — full guide for tuning
  `commands.auto_approve` / `require_approval` / `block`, profiles, MCP wiring.

### Changed
- `examples/{node,python,agent}/meerkat.yml` rewritten as **tailored**
  policies instead of byte-identical copies of `basic`. Agent example
  ships with `claude`/`codex`/`aider`/`goose` in `auto_approve`, blocks
  `.github/workflows`, includes `sandbox:` and `integrations:` opt-in
  stubs.
- README install section replaced with all three paths + Windows note +
  MCP wiring line. Linked new `docs/install.md` and `docs/auto-approval.md`.

### Fixed
- Wizard / profile no longer duplicates `claude,codex,aider,goose` entries
  when both `applyProfile("agent")` and the wizard's agent prompt run
  (added `addUnique` helper).

## [0.3.0] - 2026-05-17

### Added — Sandbox backends (opt-in via `--sandbox=` or `sandbox.enabled`)
- Backend interface + Auto selector (`internal/sandbox`).
- **macOS Seatbelt** backend via `sandbox-exec` with generated `.sb` profile.
- **Linux bubblewrap** backend with user-namespace isolation and optional `--unshare-net`.
- **Windows Job Object** marker (always-on on Windows for orphan-cleanup).
- **WSL2** re-exec backend so Windows users can use the Linux stack.
- Beta stubs for **Landlock**, **seccomp**, and **AppContainer** (report
  unavailable; native impls deferred to v0.4 pending kernel-matrix testing).
- `meerkat sandbox doctor` lists backends + which one Auto picks.
- `meerkat sandbox profile [--backend=…]` shows the generated wrap.

### Added — Network enforcement
- **Egress proxy** (`internal/sandbox/egress`): HTTP CONNECT + plain HTTP,
  with TLS SNI sniffing to verify SNI matches the CONNECT host (defeats
  domain fronting). Per-run, bound to 127.0.0.1:<random>.
- `network_egress` audit event with host, allow/deny, and reason.

### Added — Plugins (exec-based for v0.3; gRPC bus deferred)
- Plugin manager + registry (`internal/plugins`).
- **gitleaks** adapter — activates if `gitleaks` is on PATH.
- **trufflehog** adapter — activates if `trufflehog` is on PATH.
- `MergeFindings` dedup helper. Plugins can only **raise** risk;
  decision engine remains the only source of ALLOW/ASK/BLOCK.

### Added — Integrations
- **GitHub branch-protection** lookup (`internal/integrations/github`)
  with 1h cache and read-only token via env var. Remote-protected
  branches are honored by the classifier in addition to the local list.
- **MCP server** (`meerkat mcp`) — minimal JSON-RPC 2.0 over stdio,
  exposes `meerkat.explain`, `meerkat.scan`, `meerkat.approve` for
  MCP-aware coding agents.

### Added — UX
- `[meerkat] running: <cmd>` separator before child exec.
- `[meerkat] sandbox: <backend>` and `[meerkat] egress proxy: <addr>`
  status lines.

### Changed
- Policy schema gained optional `sandbox`, `plugins`, `integrations`
  sections. Absent sections preserve v0.1 behavior.
- Default Seatbelt profile uses allow-default + deny-blocked-paths
  because pure deny-default is too fragile across macOS versions for
  arbitrary CLI tools. For strict deny-default, use the Linux backend
  in a Lima/OrbStack VM.

### Fixed
- `gitguard.CurrentBranch` falls back to `git symbolic-ref --short HEAD`
  on unborn HEAD (fresh repos with no commits).
- Cross-platform process runner split (`runner_unix.go` /
  `runner_windows.go`); Windows build clean.

## [0.1.0] - 2026-05-17

### Added
- `meerkat init` generates strict default `meerkat.yml`.
- `meerkat run` supervises a child process under a policy.
- `meerkat explain` shows ALLOW / ASK / BLOCK without executing.
- `meerkat scan` runs built-in secret scanner over paths.
- `meerkat status`, `meerkat doctor`, `meerkat policy validate`.
- Deterministic rule-based command classifier.
- Decision engine (ALLOW / ASK / BLOCK) with reasons.
- Built-in secret scanner: AWS, GitHub, OpenAI, Anthropic, Stripe, Slack,
  JWT, database URLs, private keys, generic API key assignments.
- Git guard: protected-branch push block, force-push block, scan-before-commit
  and scan-before-push.
- Filesystem scope checks with symlink resolution.
- Keep-awake: macOS `caffeinate`, Linux `systemd-inhibit`. Windows is a stub.
- JSONL audit log with secret redaction.
- Dry-run mode.
- Approval prompt with timeout and `default_on_timeout: deny`.

### Security
- Default policy denies privilege escalation, network egress tools,
  recursive force-removal, and push to `main`/`master`/`production`.
