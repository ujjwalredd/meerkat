<p align="center">
  <img src="docs/assets/logo.png" alt="Meerkat" width="220"/>
</p>

<h1 align="center">Meerkat</h1>

<p align="center"><strong>Vibe coding without blind trust.</strong></p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-Apache--2.0-blue.svg" alt="License"></a>
  <a href=".github/workflows/ci.yml"><img src="https://img.shields.io/badge/ci-passing-brightgreen.svg" alt="CI"></a>
  <img src="https://img.shields.io/badge/go-1.22%2B-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/status-MVP-orange.svg" alt="Status">
</p>

Meerkat is a local CLI security wrapper for AI coding agents and long-running developer workflows. It keeps your machine awake while agents work, and only auto-approves actions that are safe, scoped, and policy-compliant. Risky actions are blocked or escalated to a human.

Meerkat is **agent-agnostic**. Wrap `claude`, `codex`, `aider`, `goose`, `npm test`, `pytest`, or any command-line tool.

> **Meerkat is not a perfect sandbox.** It is a secure-by-default local policy runner. It will become stronger over time as OS-level sandbox backends (Landlock, Seatbelt, AppContainer) and a shell-proxy mode land. Run high-risk agent work inside a VM or container too.

---

## Why Meerkat exists

AI coding agents are useful. They are also non-deterministic processes with access to your shell, your files, your network, and your git remotes. The current "approve every step" UX is exhausting and trains users to click "yes" by reflex. The "full auto" UX is dangerous: one prompt-injected README can convince an agent to read `~/.ssh`, push secrets to `main`, or `curl | sh` a malicious payload.

Meerkat draws a line:

- **Safe, scoped, deterministic actions** → auto-approve
- **Medium-risk actions** → ask once, with reasons
- **High-risk or out-of-scope actions** → block by default

The decision engine is **rule-based, not LLM-based**. Security decisions must be predictable.

Core principles:

- **Secure by default**
- **Deny by default**
- **Least privilege**
- **Scoped autonomy**
- **Human approval for high-risk actions**
- **Auditability** — every decision logged as JSONL
- **No telemetry, no cloud, no account**

---

## Install

Pick the path that matches your environment. Full guide:
[`docs/install.md`](docs/install.md).

**macOS / Linux / WSL / Git-Bash** — one-line install:

```bash
curl -fsSL https://cdn.jsdelivr.net/gh/ujjwalredd/meerkat@main/scripts/install.sh | bash
```

**All platforms (including native Windows PowerShell / cmd)** — npm wrapper:

```bash
# Interactive setup wizard — runs identically on every platform
npx meerkat-cli@latest init wizard

# Quick non-interactive init
# npx meerkat-cli@latest init

# Or install globally
npm install -g meerkat-cli
```

> 💡 **Windows users:** the `curl ... | bash` form needs a POSIX shell
> (Git-Bash, WSL, MSYS). The `npx meerkat-cli@latest init wizard` line
> works natively in PowerShell and cmd. If you hit
> `'bash' is not recognized`, use the npx line instead — both end up
> running the same init flow.

**Build from source:**

```bash
git clone https://github.com/ujjwalredd/meerkat.git
cd meerkat
go build -o /usr/local/bin/meerkat ./cmd/meerkat   # requires Go 1.22+
```

## MCP server (wire into Claude Code)

```bash
claude mcp add meerkat -- npx meerkat-cli@latest mcp start
```

Or, if installed natively:

```bash
claude mcp add meerkat -- meerkat mcp start
```

## Quick start

```bash
cd ~/my-project
meerkat init                                # write strict default meerkat.yml
meerkat run --keep-awake -- npm test        # safe + auto-approved
meerkat run -- claude                       # wrap an AI agent
meerkat explain -- git push origin main     # see the decision without running
meerkat scan                                # secret + policy scan
meerkat doctor                              # platform diagnostics
```

---

## Safe auto-approval

Meerkat resolves every command to one of three decisions, each with reasons:

| Decision | Meaning |
|----------|---------|
| `ALLOW`  | Matches `auto_approve` and `auto_approve_safe_actions: true`, or low-risk known command |
| `ASK`    | Medium risk, matches `require_approval`, or unknown command (per `mode.default_action`) |
| `BLOCK`  | Matches `commands.block`, high risk, push to protected branch, secret detected |

```text
$ meerkat explain -- git push origin main
Command: git push origin main
Decision: BLOCK
Risk: high
Reasons:
  - Push to protected branch 'main' is blocked by default
```

---

## Customizing auto-approval

Edit `commands.auto_approve`, `require_approval`, and `block` in
`meerkat.yml`. Full guide with matching rules, profiles, and recipes:
[`docs/auto-approval.md`](docs/auto-approval.md).

## Policy examples

See [`examples/`](examples/) for ready-to-use policies.

| Profile | Use case | Default |
|---------|----------|---------|
| [`basic`](examples/basic/meerkat.yml)   | Generic project | strict-but-usable |
| [`node`](examples/node/meerkat.yml)     | Node/npm app | npm test auto, install ASK |
| [`python`](examples/python/meerkat.yml) | Python/pytest | pytest auto, pip ASK |
| [`strict`](examples/strict/meerkat.yml) | CI / untrusted agent | `default_action: block`, no auto-approve |
| [`agent`](examples/agent/meerkat.yml)   | Wrapping a coding agent | auto-approve tests/lint, ASK everything else |

Full schema: [`docs/policy.md`](docs/policy.md).

---

## What gets auto-approved

Default policy auto-approves these when `auto_approve_safe_actions: true`:

- `npm test`, `npm run test`, `npm run build`, `npm run lint`
- `pnpm test`, `pnpm build`, `yarn test`
- `go test ./...`, `cargo test`, `pytest`
- `git status`, `git diff`

You edit `commands.auto_approve` to match what your team trusts.

## What requires approval

Asked once per invocation, with reasons rendered:

- `npm install`, `pnpm install`, `yarn add`, `pip install`, `poetry add`, `cargo add`, `go get`
- `git commit`, `git push` (to non-protected branches)
- `docker build`, `docker compose up`
- Any **unknown** command, when `mode.default_action: ask`
- Any network egress to a domain not on `network.allow_domains`

## What gets blocked

Refused outright. No prompt:

- `sudo`, `su`, privilege escalation
- `rm -rf /`, `rm -rf ~`, recursive force-removal
- `chmod -R 777`, `chown -R`
- `curl`, `wget`, `scp`, `ssh`, `nc`, `netcat` (default network egress tools)
- `git push --force`, `git push --force-with-lease`
- `git push` to `main`, `master`, `production`
- Reads of `~/.ssh`, `~/.aws`, `~/.config`, `~/Library/Application Support`
- Reads of `./.env`, `./.env.*`, `./secrets`, `./.git/config`
- Writes outside `filesystem.allowed_write_paths`
- Symlinks that point outside the project root

---

## Secret scanning

Built-in regex scanner. No external tools required.

Detects:

- AWS access keys (`AKIA…`, `ASIA…`)
- GitHub tokens (`ghp_…`, `gho_…`, `ghu_…`, `ghs_…`)
- OpenAI API keys (`sk-…`)
- Anthropic API keys (`sk-ant-…`)
- Slack tokens (`xoxb-…`, `xoxa-…`, `xoxp-…`)
- Stripe keys (`sk_live_…`, `pk_live_…`)
- Private key blocks (RSA, EC, DSA, OpenSSH, PGP)
- JWTs (`eyJ….….…`)
- Database URLs with embedded passwords
- Generic `api_key = "…"` / `password = "…"` assignments

Scan triggers:

- Before `git commit` if `secrets.scan_before_commit: true`
- Before `git push` if `secrets.scan_before_push: true`
- On demand via `meerkat scan`

Output:

```text
BLOCKED: Secret-like values detected

File: src/config.ts
Line: 12
Type: openai_api_key
Action: remove the secret and use environment variables instead
```

Secrets are **never printed in full**, and are redacted (`AKI**************EF`) in audit logs.

---

## Git protection

Default git guardrails:

- Push to `main`, `master`, `production` → **BLOCK**
- Force push or force-with-lease → **BLOCK**
- Commit → secret scan first (block on any finding)
- Push → secret scan first (block on any finding)
- Post-run: compare `git status` against `allowed_write_paths`; flag out-of-scope writes

Customize protected branches in `git.protected_branches`.

---

## Keep-awake mode

Long agent runs and builds make laptops sleep. Meerkat starts a keep-awake child process for the duration of the command and stops it on exit.

| OS | Backend | Status |
|----|---------|--------|
| macOS   | `caffeinate -imsd` | Supported |
| Linux   | `systemd-inhibit --what=idle:sleep` | Supported |
| Windows | `SetThreadExecutionState` | **Not implemented in MVP** — Meerkat warns honestly instead of faking it |

Modes: `disabled`, `while_command_running`, `duration`. Max duration enforced via `awake.max_duration_minutes`. Keep-awake always stops, even on Ctrl-C or panic.

---

## Audit logs

Every decision and event lands in `./.meerkat/logs/meerkat-YYYYMMDD-HHMMSS.jsonl`.

Event types: `run_started`, `command_classified`, `approval_requested`, `approval_granted`, `approval_denied`, `command_started`, `command_finished`, `secret_scan_started`, `secret_detected`, `git_guard_triggered`, `policy_violation`, `keep_awake_started`, `keep_awake_stopped`, `run_finished`.

Each event includes timestamp, command, decision, risk level, reasons, branch, exit code, duration. Secrets are redacted. Command output is **not** stored by default.

```jsonl
{"timestamp":"2026-05-17T15:04:05Z","event_type":"command_classified","command":"git push origin main","decision":"BLOCK","risk_level":"high","reasons":["Push to protected branch 'main' is blocked by default"]}
```

---

## Threat model

Full model: [`docs/threat-model.md`](docs/threat-model.md). Summary:

**Assets protected:** source code, local secrets, API keys, SSH keys, cloud credentials, git remotes, protected branches, the developer's machine.

**Adversaries:** misbehaving agent, prompt-injected agent, malicious dependency lifecycle script, careless user.

**Threats addressed:** secret reads, secret writes, network exfiltration, recursive destructive commands, out-of-scope writes, unsafe pushes, malicious installs (via ASK), symlink escape, shell injection (commands run via `exec.Command`, not a shell).

---

## Limitations

Honest list. Read before trusting:

- **Not a kernel sandbox.** Meerkat is a user-space policy runner. A malicious binary launched by an allowed command can do anything that binary is permitted to do.
- **Network controls are command/policy-based in MVP**, not packet-level. Meerkat detects `curl`, `wget`, `ssh`, `scp`, `nc`, `npm install`, `pip install`, etc., and applies domain allow/block lists at the command-line level. It does not install a firewall.
- **Secret scanner is regex-based.** False negatives are possible. Pair with `gitleaks` or `trufflehog` for high assurance.
- **Symlinks** are resolved with `EvalSymlinks` before scope checks. A TOCTOU window exists between resolution and use.
- **Anything launched outside `meerkat run` is not controlled.**
- **Windows keep-awake** is not implemented in MVP. Meerkat warns; it does not lie.
- **Do not run Meerkat with `sudo` or admin.** Least privilege starts with Meerkat itself.

For high-risk agent workloads, run Meerkat **inside** a container or VM.

---

## Roadmap

| Version | Plan |
|---------|------|
| v0.1    | CLI, policy, decisions, keep-awake, secret scan, git guard, audit |
| v0.2    | Better install detection, improved Windows, cross-platform build |
| **v0.3** | **Sandbox backend interface; macOS Seatbelt + Linux bubblewrap + WSL2 backends; HTTP CONNECT + SNI egress proxy; plugin manager + gitleaks/trufflehog adapters; MCP JSON-RPC server; GitHub branch-protection lookup** |
| v0.4    | Native Landlock + seccomp; full Windows AppContainer + per-PID firewall; gRPC plugin bus; semgrep classifier; VS Code extension |
| v1.0    | Stable policy schema, signed releases (Cosign / SLSA-3), plugin signing & registry, third-party security audit |

Detailed v1 design + per-component shipped status:
[`docs/v1-architecture.md`](docs/v1-architecture.md).

### Try v0.3 features

```bash
meerkat sandbox doctor                       # list backends, see which Auto picks
meerkat run --sandbox=auto -- npm test       # opt-in isolation
meerkat run --sandbox=seatbelt -- git status # macOS Seatbelt
meerkat mcp < requests.jsonl                 # MCP JSON-RPC 2.0 over stdio
```

```yaml
# meerkat.yml — v0.3 additions (all optional)
sandbox:
  enabled: true
  backend: auto
  fail_closed: true
  egress:
    mode: proxy
plugins:
  scanner: [gitleaks, trufflehog]
integrations:
  github:
    branch_protection_aware: true
    token_env: GITHUB_TOKEN
```

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). Ground rules:

- Security decisions stay deterministic. No LLM in the decision path.
- Default policy stays strict. Loosening defaults requires a documented threat-model rationale.
- New features that touch the network must be opt-in.
- No telemetry. Meerkat is offline-first.
- Honest documentation. Do not describe capabilities we do not have.

Report security issues privately: see [`SECURITY.md`](SECURITY.md).

---

## License

Apache-2.0. See [`LICENSE`](LICENSE).
