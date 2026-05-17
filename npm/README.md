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
meerkat run -- claude
```

First invocation downloads the Go binary for your platform from
[GitHub Releases](https://github.com/ujjwalredd/meerkat/releases) and caches
it in `~/.meerkat/bin/`. Subsequent runs exec the cached binary directly.

If no matching release asset exists, falls back to `go install` when Go 1.22+
is on PATH.

## Wire into Claude Code

```bash
claude mcp add meerkat -- npx meerkat-cli@latest mcp start
```

See the [full docs](https://github.com/ujjwalredd/meerkat#readme).

License: Apache-2.0
