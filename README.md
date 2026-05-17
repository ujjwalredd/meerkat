<p align="center">
  <img src="docs/assets/logo.png" alt="Meerkat" width="220"/>
</p>

<h1 align="center">Meerkat</h1>

<p align="center"><strong>Vibe coding without blind trust.</strong></p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue.svg" alt="License"></a>
  <a href=".github/workflows/ci.yml"><img src="https://img.shields.io/badge/ci-passing-brightgreen.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-1.22%2B-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/status-v0.4-orange.svg" alt="Status">
</p>

Local CLI security wrapper for AI coding agents and long-running developer
workflows. Keeps your machine awake while agents work. Auto-approves actions
that are safe, scoped, and policy-compliant. Blocks risky behavior. Logs every
decision.

Agent-agnostic. Wrap `claude`, `codex`, `aider`, `goose`, `npm test`,
`pytest`, or any command-line tool.

> Not a sandbox. A secure-by-default local **policy runner**. Hardens with
> OS-level backends (Seatbelt, bubblewrap, WSL2) when opted in. For
> high-risk agent workloads also run inside a container or VM.

---

## Quick start

Install the CLI:

POSIX (macOS, Linux, WSL, Git-Bash):

```bash
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | bash
```

This installs the `meerkat` binary only. It does not modify Claude Code
settings unless you opt in.

Enable the recommended Claude Code flow:

```bash
meerkat claude install
cd /path/to/your-project
meerkat init --profile=agent
```

Then use Claude Code:

```text
> /meerkat refactor the auth middleware and add tests
```

For an explicit one-command install plus Claude Code setup:

```bash
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | MEERKAT_SETUP_CLAUDE=1 bash
```

Native Windows / cross-platform:

```bash
npx meerkat-cli@latest init wizard      # interactive
npm install -g meerkat-cli              # or global install
```

From source:

```bash
go install github.com/ujjwalredd/meerkat/cmd/meerkat@latest
```

Full guide: [`docs/install.md`](docs/install.md).

What is covered:

- Claude Code CLI sessions after `meerkat claude install`.
- Direct wrappers such as `meerkat run -- npm test`.
- VS Code only when it is using a Claude Code session/tool layer that loads
  the installed Claude hooks.

What is not covered:

- Commands you run outside Meerkat or outside Claude Code hooks.
- Subprocess behavior inside already-approved commands; use containers or
  opt-in sandbox backends for high-risk workloads.

---

## Use inside Claude Code (recommended)

```bash
meerkat claude install
```

Writes a `/meerkat` slash command + Claude Code hooks (PreToolUse,
SessionStart, Stop) into `~/.claude/`. From then on:

```text
> /meerkat refactor the auth middleware and add tests
```

Behind the scenes, every Claude Code session:

- holds the laptop awake (`caffeinate` / `systemd-inhibit`),
- auto-approves safe shell commands per `meerkat.yml`,
- blocks `sudo`, `curl`, `git push origin main`, `git push --force`, etc.,
- logs every decision as JSONL.

No `meerkat run -- claude` wrapper needed. Full recipe:
[`docs/claude-integration.md`](docs/claude-integration.md).

Uninstall: `meerkat claude uninstall`.

### Other agent integration paths

Wrap any agent or command directly:

```bash
meerkat run --keep-awake -- claude
meerkat run -- npm test
meerkat explain -- git push origin main         # dry-decision
meerkat run --dry-run -- "npm install left-pad" # see decision, don't execute
```

MCP server for MCP-aware agents:

```bash
claude mcp add meerkat -- meerkat mcp start
```

See [`docs/agent-integrations.md`](docs/agent-integrations.md).

---

## Decisions

Every command resolves deterministically to one of:

| Decision | Trigger |
|----------|---------|
| `ALLOW`  | matches `commands.auto_approve` and `auto_approve_safe_actions: true` |
| `ASK`    | matches `require_approval`, or unknown command (per `mode.default_action`) |
| `BLOCK`  | matches `commands.block`, high-risk heuristic, protected branch push, secret detected |

Rule-based, no LLM in the security path.

---

## Quick reference

```bash
meerkat init [wizard] [--profile=basic|strict|agent|node|python]
meerkat run [--policy F] [--keep-awake] [--sandbox=auto|seatbelt|bwrap|wsl2|off]
            [--dry-run] -- <cmd>
meerkat scan [paths...]                 # secret + policy scan
meerkat status                          # workspace / policy / branch / awake
meerkat doctor                          # platform diagnostics
meerkat sandbox doctor                  # available isolation backends
meerkat policy validate
meerkat explain -- <cmd>                # decision without executing
meerkat mcp [start]                     # JSON-RPC MCP server on stdio
meerkat claude install|uninstall        # /meerkat slash cmd + hooks
meerkat hook pretooluse|sessionstart|stop
meerkat version
```

---

## Default policy protects against

- Reading `.env`, `~/.ssh`, `~/.aws`, cloud credentials
- Committing or pushing secret-like values (built-in scanner)
- Push to `main` / `master` / `production`
- `git push --force` / `--force-with-lease`
- `sudo`, `su`, `rm -rf /`, recursive `chmod`/`chown`
- `curl`, `wget`, `ssh`, `scp`, `nc` network egress
- Writes outside `filesystem.allowed_write_paths`
- Symlink escape from project root

Does **not** protect against: kernel exploits, compromised toolchain,
packet-level egress, malicious binary spawned by an allowed command,
anything launched outside `meerkat run` or Claude Code hooks. Full
threat model: [`docs/threat-model.md`](docs/threat-model.md).

---

## Customize auto-approval

Edit `commands.auto_approve`, `require_approval`, `block` in `meerkat.yml`.
Matching rules, profiles, recipes:
[`docs/auto-approval.md`](docs/auto-approval.md).

Full schema: [`docs/policy.md`](docs/policy.md).

Example policies: [`examples/`](examples/) ·
[`docs/examples.md`](docs/examples.md).

---

## Sandbox + plugins + MCP (v0.3)

Opt-in OS isolation, external scanners, MCP server, GitHub
branch-protection awareness. Design + shipped status:
[`docs/v1-architecture.md`](docs/v1-architecture.md).

```yaml
# meerkat.yml — v0.3 additions (all optional)
sandbox:
  enabled: true
  backend: auto                # seatbelt (macOS), bwrap (Linux), wsl2 (Windows)
  fail_closed: true
  egress:
    mode: proxy                # HTTP CONNECT + SNI-sniffing forward proxy
plugins:
  scanner: [gitleaks, trufflehog]
integrations:
  github:
    branch_protection_aware: true
    token_env: GITHUB_TOKEN
```

---

## Roadmap

| Version | Plan |
|---------|------|
| v0.1 | CLI, policy, decisions, keep-awake, secret scan, git guard, audit |
| v0.2 | Cross-platform build, install paths |
| v0.3 | Sandbox backends (Seatbelt / bubblewrap / WSL2), egress proxy, plugin manager + gitleaks/trufflehog, MCP server, GitHub branch-protection, Claude Code hooks (`/meerkat`) |
| v0.4 | Claude Code file-tool enforcement, safer installer onboarding, hook preservation, policy hardening |
| v0.5 | Native Landlock + seccomp; AppContainer + per-PID firewall; shell-proxy mode; semgrep classifier; VS Code extension |
| v1.0 | Stable policy schema, signed releases (Cosign / SLSA-3), plugin signing, third-party security audit |

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md) and [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md).
Security disclosures: [`SECURITY.md`](SECURITY.md).

## License

Apache-2.0. See [`LICENSE`](LICENSE).
