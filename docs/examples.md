# Examples

See [`examples/`](../examples) for full policy files.

## Wrap an AI coding agent

```bash
meerkat init
meerkat run --policy meerkat.yml --keep-awake -- claude
```

Result: keep-awake starts via `caffeinate` (macOS) or `systemd-inhibit` (Linux),
agent runs, audit log captures every classified command, keep-awake stops on
exit.

## Long build

```bash
meerkat run --keep-awake -- npm run build
```

## Dry-run

```bash
meerkat run --dry-run -- "npm install left-pad"
```

Prints decision + reasons, executes nothing.

## Explain a command

```bash
meerkat explain -- git push origin main
# Decision: BLOCK
# Risk: high
# Reasons:
#   - Push to protected branch 'main' is blocked by default
```

## Secret scan a repo

```bash
meerkat scan
meerkat scan --json src/
```

## Strict CI-grade policy

Use [`examples/strict/meerkat.yml`](../examples/strict/meerkat.yml):
`default_action: block`, no auto-approve, no network egress, no protected push.
