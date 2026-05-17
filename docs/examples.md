# Examples

Five ready-to-use starter policies. Generate any of them with
`meerkat init --profile=<name>`.

| File | When to use |
|------|------|
| [`examples/basic/meerkat.yml`](../examples/basic/meerkat.yml)     | Generic project, strict-but-usable defaults |
| [`examples/strict/meerkat.yml`](../examples/strict/meerkat.yml)   | CI / untrusted agent — `default_action: block`, no auto-approve, no network egress |
| [`examples/agent/meerkat.yml`](../examples/agent/meerkat.yml)     | Wrapping an AI coding agent — auto-approves `claude`/`codex`/`aider`/`goose` + tests, blocks `.github/workflows`, sandbox + egress proxy opt-in |
| [`examples/node/meerkat.yml`](../examples/node/meerkat.yml)       | Node / npm / pnpm — `npm test`, `npx`, `tsc`, `vitest`, `jest` auto-approved |
| [`examples/python/meerkat.yml`](../examples/python/meerkat.yml)   | Python / pip / poetry — `pytest`, `ruff`, `mypy` auto-approved |

## Common recipes

```bash
# wrap an AI agent with one shot
meerkat init --profile=agent
meerkat run --keep-awake -- claude

# native Claude Code integration
meerkat claude install
# then in Claude Code: /meerkat refactor the auth middleware

# dry-run any decision without executing
meerkat run --dry-run -- "npm install left-pad"

# audit the current branch for secrets
meerkat scan

# CI policy gate
meerkat explain -- git push origin main && echo allowed || echo blocked
```
