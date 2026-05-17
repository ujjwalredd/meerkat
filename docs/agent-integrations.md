# Agent integrations

Meerkat is **agent-agnostic**. Three integration paths.

## 1. Claude Code (recommended) — `/meerkat <prompt>`

`meerkat claude install` writes a slash command + hooks. Every Claude
Code session then runs under Meerkat automatically: keep-awake on,
shell commands auto-approved/blocked per policy. Full recipe:
[`claude-integration.md`](claude-integration.md).

## 2. Wrap any agent with `meerkat run`

```bash
meerkat run --keep-awake -- claude
meerkat run --keep-awake -- codex
meerkat run --keep-awake -- aider
meerkat run --keep-awake -- goose
meerkat run --keep-awake -- npm run agent
```

Wraps the outer agent process. Inner shell commands the agent spawns
are **not** classified in this mode (shell-proxy is planned for v0.5); use
path 1 or 3 for per-command enforcement.

## 3. MCP server — any MCP-aware agent

```bash
meerkat mcp start              # JSON-RPC 2.0 on stdio
# wire into Claude Code:
claude mcp add meerkat -- meerkat mcp start
```

| Method | Params | Returns |
|--------|--------|---------|
| `meerkat.explain` | `{"command":"git push origin main"}` | `{"decision":"BLOCK","risk_level":"high","reasons":[...]}` |
| `meerkat.scan`    | `{"paths":["./src"]}`                | array of `{file,line,type,redacted}` findings |
| `meerkat.approve` | `{"command":"..."}`                  | decision + note saying meerkat never auto-approves through MCP |

One-shot example:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"meerkat.explain","params":{"command":"git push origin main"}}' \
  | meerkat mcp
```

## Today vs. future

| Capability | Status |
|---|---|
| Wrap outer agent process | shipped (`meerkat run`) |
| Per-tool enforcement via Claude Code hooks | shipped (`meerkat claude install`) |
| MCP `explain`/`scan`/`approve` | shipped (`meerkat mcp`) |
| Shell-proxy mode (intercept every inner shell) | v0.5 |
| VS Code / Cursor extension | v0.5 (reuses MCP) |
