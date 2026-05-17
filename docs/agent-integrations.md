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
| Intercept every inner shell command the agent runs | No (v0.3) |
| Tool-call approval API for agent SDKs | No (v0.3) |
| MCP approval server | No (v0.3) |

## Future (v0.3+)

- **Shell proxy mode.** Set the agent's shell to a MeerKat-supervised shim
  that runs each child command through the decision engine.
- **MCP server.** Expose `approve` / `deny` / `explain` as MCP tools so an
  agent can request approval inline.
- **Tool-call adapter.** For agents with structured tool calls (write_file,
  run_shell), check the call against the policy before the tool executes.
- **VS Code / Cursor extension.** Inline approval UI.

## Recommended pattern

1. Run `meerkat init` in your project.
2. Tune `commands.auto_approve` for what your team trusts (`npm test`,
   `pytest`, `go test`).
3. Leave `git push`, `npm install`, and unknown commands as ASK.
4. Keep `mode.default_action: ask` while you learn what the agent does, then
   tighten to `block` once you trust your auto-approve list.
