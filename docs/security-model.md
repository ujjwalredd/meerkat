# Security model

Meerkat is a local policy runner for AI coding workflows. It is designed to
reduce accidental or prompt-injected damage while preserving a normal developer
loop.

## Security boundary

Meerkat can enforce decisions only where it is in the execution path:

- `meerkat run -- <command>`
- Claude Code hooks installed by `meerkat claude install`
- MCP-aware agents that call `meerkat mcp`

Commands launched outside those paths are outside Meerkat's control.

## What Meerkat does

- Classifies shell commands with deterministic local rules.
- Auto-approves only low-risk commands that match policy.
- Asks for approval on unknown or medium-risk commands.
- Blocks high-risk commands such as privilege escalation, protected-branch
  pushes, force pushes, recursive force-removal, and blocked network tools.
- Checks Claude Code file read/write tools against allowed and blocked paths.
- Scans staged or changed files for secret-like values before commit/push when
  enabled.
- Logs decisions as JSONL and redacts built-in secret patterns from audit
  events when `audit.redact_secrets` is enabled.
- Optionally wraps commands in OS sandbox backends when configured.

## What Meerkat does not protect against

- Kernel exploits or host compromise.
- Malicious compilers, interpreters, package lifecycle scripts, or binaries
  that run after an allowed command starts.
- Network egress from tools that bypass proxy environment variables unless an
  OS sandbox or external firewall also blocks direct egress.
- Browser, keychain, and desktop-app secrets outside the configured file scope.
- Commands run directly in another terminal outside Meerkat.
- A user approving a dangerous action.

## Default decision model

The decision engine returns one of:

- `ALLOW`: known low-risk command and auto-approval is enabled.
- `ASK`: unknown, medium-risk, or explicitly approval-required command.
- `BLOCK`: high-risk command, blocked path, protected branch push, force push,
  or secret finding.

There is no LLM in the security path.

## Recommended hardening

- Commit a project-specific `meerkat.yml`.
- Keep `mode.default_action: ask` or stricter.
- Keep `mode.deny_out_of_scope: true`.
- Enable secret scanning before commit and push.
- Use `meerkat claude install` for Claude Code.
- Use `meerkat run --sandbox=auto -- <command>` or a VM/container for
  high-risk tasks.
- Install from tagged GitHub Releases and verify checksums for sensitive
  environments.

Full threat model: [`docs/threat-model.md`](threat-model.md). Known
limitations: [`docs/known-limitations.md`](known-limitations.md).
