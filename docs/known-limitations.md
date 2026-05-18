# Known limitations

Meerkat is a policy runner, not a complete host security boundary. This page is
intentionally direct so users know where the tool helps and where they still
need OS isolation, a container, a VM, or manual review.

## Execution scope

Meerkat can enforce policy only when it is in the execution path:

- `meerkat run -- <command>`
- Claude Code hooks installed by `meerkat claude install`
- MCP-aware agents that explicitly call Meerkat

Commands launched in another terminal, another editor integration, a browser,
or an already-running process are outside Meerkat's control.

## Shell classification

Meerkat parses shell commands with `mvdan.cc/sh/v3/syntax` and inspects command
AST nodes instead of matching raw substrings. This avoids common false matches
like `echo "curl ..."`, but it still cannot prove every runtime behavior.

Important cases that still require approval or extra isolation:

- Variable expansion, command substitution, aliases, shell functions, and PATH
  lookup can change what actually runs.
- A low-risk command can invoke scripts, test helpers, package hooks, or child
  processes after it starts.
- Malformed shell input is treated conservatively with a warning and a
  fallback tokenization path.

## Sandboxing

Sandbox backends are opt-in and platform-dependent:

- macOS uses Seatbelt profiles where available.
- Linux can use bubblewrap when installed and supported by the host.
- Windows support is limited to the configured backend path, such as WSL2.

If a sandbox backend is unavailable and `sandbox.fail_closed` is not enabled,
Meerkat can still make policy decisions but cannot provide OS isolation.

## Network control

The egress proxy helps with tools that honor proxy environment variables. It is
not a packet filter. Tools that bypass proxy variables, use raw sockets, or run
outside the supervised process tree need an OS firewall, container network
policy, VM, or sandbox backend.

## Secret scanning

The built-in scanner is heuristic. It catches common secret-like values and
blocked paths, but it is not a replacement for provider-side secret scanning,
pre-receive hooks, or a dedicated scanner such as gitleaks or trufflehog.

## Claude Code hooks

Claude integration depends on Claude Code loading the installed hook files and
honoring hook responses. Run `meerkat doctor` after install and after Claude
Code updates. If the hook protocol changes, Meerkat may need a compatibility
release.

## Releases and installers

The installer downloads GitHub Release assets and verifies `checksums.txt` when
available. For stricter environments, use:

```bash
MEERKAT_REQUIRE_CHECKSUM=1 \
MEERKAT_INSTALL_NO_GO_FALLBACK=1 \
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | bash
```

Maintainers should run `meerkat doctor --release` from a release build after
publishing a tag.
