# Agent integrations

MeerKat is agent-agnostic. Any agent or workflow that runs as a child process
can be wrapped:

```bash
meerkat run -- claude
meerkat run -- codex
meerkat run -- aider
meerkat run -- goose
meerkat run -- npm run agent
```

For MVP, MeerKat supervises the **outer** process. It classifies what the
outer process is, not every shell command the agent decides to run internally.
A future shell-proxy mode will intercept inner commands too.

## Today (MVP)

| Capability | Supported |
|---|---|
| Wrap and supervise outer agent process | Yes |
| Keep-awake during agent run | Yes (macOS, Linux) |
| Audit log of agent invocation | Yes |
| Block agent launch if policy rejects | Yes |
| Intercept every inner shell command the agent runs | No (v0.4, shell-proxy) |
| Tool-call approval API for agent SDKs | Partial via MCP (v0.3) |
| MCP approval server | **Yes (v0.3)** — `meerkat mcp` |

## MCP server (shipped in v0.3)

```bash
meerkat mcp --policy meerkat.yml
```

Reads line-delimited JSON-RPC 2.0 from stdin, writes responses to stdout.
Methods:

| Method | Params | Returns |
|--------|--------|---------|
| `meerkat.explain` | `{ "command": "git push origin main" }` | `{ "decision": "BLOCK", "risk_level": "high", "reasons": [...] }` |
| `meerkat.scan`    | `{ "paths": ["./src"] }`              | array of `{file,line,type,redacted}` findings |
| `meerkat.approve` | `{ "command": "..." }`                | decision + note saying "Agent must surface this to the user; meerkat never auto-approves through MCP." |

Example one-shot:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"meerkat.explain","params":{"command":"git push origin main"}}' \
  | meerkat mcp
```

## Future (v0.4+)

- **Shell proxy mode.** Set the agent's shell to a MeerKat-supervised shim
  that runs each child command through the decision engine.
- **Tool-call adapter.** For agents with structured tool calls (write_file,
  run_shell), check the call against the policy before the tool executes.
- **VS Code / Cursor extension.** Inline approval UI, reuses the MCP server.

## Recommended pattern

1. Run `meerkat init` in your project.
2. Tune `commands.auto_approve` for what your team trusts (`npm test`,
   `pytest`, `go test`).
3. Leave `git push`, `npm install`, and unknown commands as ASK.
4. Keep `mode.default_action: ask` while you learn what the agent does, then
   tighten to `block` once you trust your auto-approve list.
