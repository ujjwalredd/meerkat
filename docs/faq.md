# FAQ

## Does the curl installer enable `/meerkat` automatically?

No. The default installer only installs the `meerkat` binary. Enable Claude
Code integration explicitly:

```bash
meerkat claude install
```

For a one-command install plus Claude setup:

```bash
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | MEERKAT_SETUP_CLAUDE=1 bash
```

## Why is Claude setup opt-in?

Because it writes to `~/.claude/settings.json` and installs hooks. A security
tool should not silently change agent behavior from a curl installer.

## Does Meerkat work in VS Code?

Only when VS Code is using a Claude Code session/tool layer that loads the
installed Claude hooks. Direct terminal commands are covered only if you run
them through:

```bash
meerkat run -- <command>
```

## Why did Meerkat ask for approval on `npm test && git status`?

Shell chains, pipes, and redirects are treated as medium risk. Each segment is
inspected, but the combined command can have broader side effects than a single
known command.

## Why is `bash -c "npm test"` not auto-approved?

Shell wrappers hide the real command from simple prefix policies. Meerkat
inspects the inner command, but still asks because `sh -c` changes the
execution shape.

## Does Meerkat block all network traffic?

No. It blocks known network tools at the policy layer and can run an optional
HTTP(S) egress proxy. Direct network egress from arbitrary binaries requires an
OS sandbox, firewall, container, or VM.

## How do I verify an installed binary?

GitHub Releases include `checksums.txt`. The installer verifies checksums when
available. For stricter installs:

```bash
MEERKAT_REQUIRE_CHECKSUM=1 \
MEERKAT_INSTALL_NO_GO_FALLBACK=1 \
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | bash
```

See [`docs/release.md`](release.md) for Cosign and GitHub attestation
verification. Maintainers can run `meerkat doctor --release` after publishing
to check every expected release asset.

## What should I do if `meerkat doctor` warns about PATH?

Add the install directory to your shell profile. The installer prints the exact
`export PATH=...` line when it installs into a directory not currently in
`PATH`.

## Can I remove the Claude integration?

Yes:

```bash
meerkat claude uninstall
```

This removes Meerkat's slash command and hooks while preserving unrelated
Claude settings.
