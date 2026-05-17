# Customizing auto-approval

Meerkat splits every command into three buckets via your `meerkat.yml`:

| Bucket | When fired | Default behavior |
|--------|-----------|------------------|
| `commands.auto_approve`     | safe, repeatable, no network surprises | run without prompt (if `mode.auto_approve_safe_actions: true`) |
| `commands.require_approval` | medium risk — touches deps, commits, network | prompt the user once with reasons |
| `commands.block`            | dangerous — never run, no prompt | refuse and exit non-zero |

Plus a fallback for everything else: `mode.default_action` (`ask` | `allow` | `block`).

Decision priority is **deterministic** and documented in
[`policy.md`](policy.md). The same command produces the same decision every
time. No LLM in the security path.

---

## How to add a command to auto-approve

Edit `meerkat.yml`:

```yaml
commands:
  auto_approve:
    - npm test
    - go test ./...
    - claude              # add the agent itself
    - codex
    - my-internal-script
```

Matching rules:

- **Whole-token prefix.** `npm test` matches `npm test`, `npm test --watch`,
  `npm test -- --grep auth`. It does **not** match `npm testfoo`.
- **Single-token patterns** like `claude` or `sudo` match the binary
  invocation, including path-qualified binaries such as `/usr/bin/sudo`.
- **Case-insensitive.** `Git` and `git` are equivalent.
- **Shell-aware.** Quotes are respected, so `echo "curl"` does not match the
  blocked `curl` command. Shell chains, pipes, redirects, and `sh -c` wrappers
  require approval even when one segment matches an auto-approve pattern.

Verify before trusting it:

```bash
meerkat explain -- npm test
# Decision: ALLOW
# Reasons:  Command matches auto-approve pattern: npm test
```

---

## How to add a command to require approval

```yaml
commands:
  require_approval:
    - npm install
    - pip install
    - poetry add
    - docker compose up
    - git push                # prompts only when not pushing to a protected branch
```

A `require_approval` match always prompts (timeout 120s, default `deny`).
Approval choices:

- `y` — approve this one invocation
- `s` — approve for the whole session
- `n` — deny
- `N` — always deny this pattern (persists for the session)

---

## How to harden block-list

```yaml
commands:
  block:
    - sudo
    - su
    - rm -rf /
    - rm -rf ~
    - chmod -R 777
    - curl
    - wget
    - ssh
    - scp
    - nc
    - powershell Invoke-WebRequest
```

`block` wins over `auto_approve` and `require_approval`. There is **no
prompt** for blocked commands.

---

## Auto-approve AI agents (Claude, Codex, Aider, Goose)

`meerkat init --profile=agent` pre-seeds these. Manually:

```yaml
commands:
  auto_approve:
    - claude
    - codex
    - aider
    - goose
```

Now `meerkat run -- claude` skips the approval prompt, starts the agent
cleanly (no prompt noise above its TUI), and keep-awake holds the laptop
open while the agent works.

The agent's **inner** shell commands are not yet intercepted by Meerkat
(shell-proxy mode is planned for v0.5). For per-tool-call approval today, use
the MCP server:

```bash
claude mcp add meerkat -- meerkat mcp start
```

The agent can then call `meerkat.explain` / `meerkat.approve` inline before
running a tool. See [`claude-integration.md`](claude-integration.md).

---

## Per-project policy

`meerkat.yml` lives in the project root and is loaded automatically. Commit
it to the repo so the whole team — and the agents wrapped by `meerkat run`
— see the same policy.

To use a non-default path:

```bash
meerkat run --policy /path/to/custom.yml -- npm test
```

---

## Profiles

`meerkat init --profile=<name>` writes a tailored starter:

| Profile | What changes |
|---------|--------------|
| `basic`   | default strict-but-usable policy |
| `strict`  | `default_action: block`, no auto-approve, no network egress |
| `agent`   | adds `claude`, `codex`, `aider`, `goose` to auto-approve; blocks `.github/workflows`; sandbox opt-in stubs filled in |
| `node`    | adds `node`, `npx`, `tsc`, `vitest`, `jest` |
| `python`  | adds `python`, `python3`, `pytest -q`, `ruff`, `mypy` |

Interactive variant: `meerkat init wizard` asks a handful of questions
(project name, profile, whether to wrap agents, whether to enable sandbox
or egress proxy) and writes the matching policy.

---

## Verify any decision without running it

```bash
meerkat explain -- git push origin main
meerkat explain -- "rm -rf ./build"
meerkat explain -- claude --resume
```

Exit code:

| Code | Decision |
|------|----------|
| `0`  | ALLOW |
| `4`  | BLOCK |
| `6`  | ASK |

Useful in CI hooks to refuse to even consider running a command that would
not pass the policy.
