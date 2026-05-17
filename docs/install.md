# Install

Three ways to install Meerkat. Pick whichever matches your environment.

---

## 1. One-line install (POSIX shells)

macOS, Linux, WSL, Git-Bash, MSYS:

```bash
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | bash
```

What it does:

- Detects OS (`darwin`/`linux`/`windows`) and arch (`amd64`/`arm64`).
- Resolves the latest release from
  `https://github.com/ujjwalredd/meerkat/releases/latest`.
- Downloads the matching prebuilt binary.
- Verifies the binary against release `checksums.txt` when available.
- Installs to `/usr/local/bin` (if writable) or `~/.local/bin`.
- Falls back to `go install` if no prebuilt asset matches but Go 1.22+ is on PATH.
- Prints the next steps for Claude Code and project policy setup.

What it does not do by default:

- It does not edit `~/.claude/settings.json`.
- It does not create a project policy file.
- It does not protect commands run outside Meerkat or Claude Code hooks.

Environment overrides:

| Env var | Purpose |
|---------|---------|
| `MEERKAT_VERSION=v0.4.1` | Pin a specific release |
| `INSTALL_DIR=/path/to/bin` | Custom install location |
| `MEERKAT_REPO=owner/name` | Use a fork |
| `MEERKAT_SETUP_CLAUDE=1` | Also run `meerkat claude install` after installing the binary |
| `MEERKAT_REQUIRE_CHECKSUM=1` | Fail if release checksums are unavailable |
| `MEERKAT_INSTALL_NO_GO_FALLBACK=1` | Fail instead of falling back to `go install` |

Explicit one-command CLI + Claude Code setup:

```bash
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | MEERKAT_SETUP_CLAUDE=1 bash
```

This is opt-in because it writes Claude Code hooks and the `/meerkat` slash
command under `~/.claude/`.

---

## 2. npm / npx (all platforms incl. native Windows)

Works on PowerShell, cmd, every shell:

```bash
# Interactive setup wizard
npx meerkat-cli@latest init wizard

# Quick non-interactive
npx meerkat-cli@latest init

# Install globally
npm install -g meerkat-cli
```

First run downloads the Go binary for the host platform from GitHub Releases
and caches it in `~/.meerkat/bin/`. Subsequent runs exec the cached binary
directly — no Node overhead per command.

> **Windows note:** the `curl … | bash` form needs a POSIX shell (Git-Bash,
> WSL, MSYS). The `npx meerkat-cli@latest init wizard` line works natively
> in PowerShell and cmd. If you hit `'bash' is not recognized`, use the npx
> line — both end up running the same init flow.

---

## 3. Build from source

```bash
git clone https://github.com/ujjwalredd/meerkat.git
cd meerkat
go build -o /usr/local/bin/meerkat ./cmd/meerkat
```

Requires Go 1.22+. Cross-compile for another platform:

```bash
GOOS=linux  GOARCH=amd64 go build -o meerkat-linux-amd64  ./cmd/meerkat
GOOS=darwin GOARCH=arm64 go build -o meerkat-darwin-arm64 ./cmd/meerkat
GOOS=windows GOARCH=amd64 go build -o meerkat-windows-amd64.exe ./cmd/meerkat
```

---

## Enable Claude Code `/meerkat`

After installing the binary:

```bash
meerkat claude install
cd /path/to/your-project
meerkat init --profile=agent
```

Then open Claude Code in that project and run:

```text
> /meerkat fix the bug and run tests
```

This writes the `/meerkat` slash command plus PreToolUse, SessionStart, and
Stop hooks into `~/.claude/`. Safe commands can be auto-approved by policy,
risky commands are asked or blocked, and macOS/Linux sessions can be kept
awake while work runs.

VS Code is covered only when it is using a Claude Code session/tool layer
that loads those Claude hooks. Commands run directly in a terminal are covered
only when you call them through `meerkat run -- <command>`.

## Optional MCP server

```bash
claude mcp add meerkat -- npx meerkat-cli@latest mcp start
```

Or, if you installed natively:

```bash
claude mcp add meerkat -- meerkat mcp start
```

Claude (and any MCP-aware agent) can then call `meerkat.explain`,
`meerkat.scan`, and `meerkat.approve` inline. See
[`docs/claude-integration.md`](claude-integration.md) for the full flow.

---

## Verify

```bash
meerkat version            # → meerkat 0.4.1
meerkat doctor             # → PATH, Claude hooks, policy, sandbox, keep-awake
meerkat doctor --online    # → also checks GitHub release asset availability
meerkat sandbox doctor     # → available sandbox backends, what Auto picks
```

---

## Uninstall

```bash
# native
rm $(which meerkat)
rm -rf ~/.meerkat

# npm
npm uninstall -g meerkat-cli
rm -rf ~/.meerkat

# MCP wiring
claude mcp remove meerkat

# Claude Code hooks + /meerkat slash command
meerkat claude uninstall
```
