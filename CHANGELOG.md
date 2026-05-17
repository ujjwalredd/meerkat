# Changelog

All notable changes follow [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-05-17

### Added
- `meerkat init` generates strict default `meerkat.yml`.
- `meerkat run` supervises a child process under a policy.
- `meerkat explain` shows ALLOW / ASK / BLOCK without executing.
- `meerkat scan` runs built-in secret scanner over paths.
- `meerkat status`, `meerkat doctor`, `meerkat policy validate`.
- Deterministic rule-based command classifier.
- Decision engine (ALLOW / ASK / BLOCK) with reasons.
- Built-in secret scanner: AWS, GitHub, OpenAI, Anthropic, Stripe, Slack,
  JWT, database URLs, private keys, generic API key assignments.
- Git guard: protected-branch push block, force-push block, scan-before-commit
  and scan-before-push.
- Filesystem scope checks with symlink resolution.
- Keep-awake: macOS `caffeinate`, Linux `systemd-inhibit`. Windows is a stub.
- JSONL audit log with secret redaction.
- Dry-run mode.
- Approval prompt with timeout and `default_on_timeout: deny`.

### Security
- Default policy denies privilege escalation, network egress tools,
  recursive force-removal, and push to `main`/`master`/`production`.
