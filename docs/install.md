# Install

Three ways to install Meerkat. Pick whichever matches your environment.

---

## 1. One-line install (POSIX shells)

macOS, Linux, WSL, Git-Bash, MSYS:

```bash
curl -fsSL https://cdn.jsdelivr.net/gh/ujjwalredd/meerkat@main/scripts/install.sh | bash
```

What it does:

- Detects OS (`darwin`/`linux`/`windows`) and arch (`amd64`/`arm64`).
- Resolves the latest release from
  `https://github.com/ujjwalredd/meerkat/releases/latest`.
- Downloads the matching prebuilt binary.
- Installs to `/usr/local/bin` (if writable) or `~/.local/bin`.
- Falls back to `go install` if no prebuilt asset matches but Go 1.22+ is on PATH.

Environment overrides:

| Env var | Purpose |
|---------|---------|
| `MEERKAT_VERSION=v0.3.0` | Pin a specific release |
| `INSTALL_DIR=/path/to/bin` | Custom install location |
| `MEERKAT_REPO=owner/name` | Use a fork |

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

## Wire into Claude Code (MCP)

```bash
claude mcp add meerkat -- npx meerkat-cli@latest mcp start
```

Or, if you installed natively:

```bash
claude mcp add meerkat -- meerkat mcp start
```

Claude (and any MCP-aware agent) can then call `meerkat.explain`,
`meerkat.scan`, and `meerkat.approve` inline. See
[`docs/agent-integrations.md`](agent-integrations.md) for method signatures.

---

## Verify

```bash
meerkat version            # → meerkat 0.3.0
meerkat doctor             # → OS, git, keep-awake backend, policy validity
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
```
