# Architecture

```
cmd/meerkat/main.go              CLI entry: init|run|scan|status|doctor|policy|
                                 explain|sandbox|mcp|version
internal/
  config/                        Policy file load + validate + default
                                 (incl. v0.3 sandbox/plugins/integrations)
  commandpolicy/                 Risk classifier (rule-based, deterministic)
  decision/                      classification + mode → ALLOW|ASK|BLOCK
  filesystem/                    Path resolution, symlink-safe scope checks
  scanner/                       Built-in regex secret scanner
  gitguard/                      git status / staged / branch helpers
  networkpolicy/                 Domain extraction + allow/block evaluation
  awake/                         caffeinate / systemd-inhibit
  processrunner/                 exec + signal forwarding (unix/windows split)
  audit/                         JSONL audit logger
  ui/                            Approval prompt
  sandbox/                       Backend interface + Auto selector
    seatbelt/                    macOS sandbox-exec (functional)
    bwrap/                       Linux bubblewrap (functional)
    landlock/                    Linux Landlock LSM (beta stub)
    seccomp/                     Linux seccomp-bpf (beta stub)
    jobobject/                   Windows Job Object (always-on)
    appcontainer/                Windows AppContainer (beta stub)
    wsl2/                        Windows → WSL2 re-exec
    egress/                      HTTP CONNECT + SNI-sniffing forward proxy
  plugins/                       Exec-based plugin manager
                                 (gitleaks, trufflehog adapters)
  integrations/github/           Branch-protection lookup (1h cache)
  mcp/                           JSON-RPC 2.0 over stdio MCP server
```

## Lifecycle of `meerkat run -- <cmd>`

1. Load and validate policy.
2. Open audit log; emit `run_started`.
3. Classify command → risk + reasons.
4. Decide ALLOW / ASK / BLOCK (deterministic).
5. If `git commit`/`push` and policy requires it → secret scan first.
6. If BLOCK → log `policy_violation`, exit non-zero.
7. If ASK → render approval prompt with timeout.
8. Start keep-awake (if enabled) — `caffeinate` or `systemd-inhibit`.
9. Run command with inherited stdio, signal forwarding.
10. Post-run: compare `git status` to allowed write paths.
11. Stop keep-awake; emit `run_finished`; exit with child's code.

## Why deterministic

Security decisions in MeerKat are rule-based on purpose. An LLM might be wrong
in surprising ways; a regex is wrong in predictable ways. The classifier and
decision engine are fully testable.

## Why agent-agnostic

MeerKat wraps the **outer process**. It does not parse the agent's tool-call
protocol (yet). Whatever the agent runs as a child process passes through the
same OS-level command interface, which MeerKat can supervise via shell-proxy
mode in a future release.
