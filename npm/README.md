# meerkat-cli

NPM wrapper for [Meerkat](https://github.com/ujjwalredd/meerkat) — local CLI
security wrapper for AI coding agents.

```bash
# Interactive setup
npx meerkat-cli@latest init

# MCP server (for Claude Code, Cursor, any MCP-aware agent)
npx meerkat-cli@latest mcp start

# Or install globally
npm install -g meerkat-cli
meerkat claude install
```

First invocation downloads the Go binary for your platform from
[GitHub Releases](https://github.com/ujjwalredd/meerkat/releases) and caches
it in `~/.meerkat/bin/`. Subsequent runs exec the cached binary directly.

If no matching release asset exists, falls back to `go install` when Go 1.22+
is on PATH.

## Wire into Claude Code

Recommended `/meerkat` slash-command flow after global install:

```bash
meerkat claude install
cd /path/to/your-project
meerkat init --profile=agent
```

Then use Claude Code:

```text
> /meerkat fix the bug and run tests
```

Optional MCP server:

```bash
claude mcp add meerkat -- npx meerkat-cli@latest mcp start
```

See the [full docs](https://github.com/ujjwalredd/meerkat#readme).

License: Apache-2.0
