# Meerkat v1 Architecture — Stronger Isolation, Plugins, Integrations

> Status: design document. Targets v0.3 → v1.0. Default experience stays simple;
> advanced isolation is opt-in and only after each backend is stable.

---

## 1. Design principles (still hold at v1)

- Secure by default. Deny by default. Least privilege.
- Deterministic, rule-based security decisions. No LLM in the decision path.
- Auditability of every decision and every isolation event.
- Opt-in sandbox backends. **`meerkat init` never enables an unstable backend.**
- Honest degradation. If a backend is unavailable, Meerkat says so; it does
  not silently fall back to weaker enforcement without telling the user.
- Single static binary. No daemon required for MVP-equivalent features.
- Plugins are out-of-process by default. A buggy plugin must not crash the
  core decision engine.

## 2. Layered architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│  CLI / IDE extension / MCP server / shell-proxy shim                 │  Front ends
└──────────────────────────────────────────────────────────────────────┘
                              │
┌──────────────────────────────▼──────────────────────────────────────┐
│  Decision Engine (deterministic, rule-based)                        │
│  - classifier        - decision           - scope checks            │
│  - policy loader     - approval state     - audit emit              │
└──────────────────────────────┬──────────────────────────────────────┘
                               │  enforce
┌──────────────────────────────▼──────────────────────────────────────┐
│  Enforcement Layer                                                  │
│  ┌────────────────────────┐  ┌─────────────────────────────────┐    │
│  │ Process Runner          │  │ Sandbox Backend (opt-in)       │    │
│  │ (always present)        │  │  - macos.seatbelt              │    │
│  │ - exec.Command          │  │  - linux.landlock              │    │
│  │ - signal forwarding     │  │  - linux.seccomp               │    │
│  │ - pgroup cleanup        │  │  - linux.bubblewrap            │    │
│  └────────────────────────┘  │  - linux.egress_proxy           │    │
│                              │  - windows.appcontainer         │    │
│                              │  - windows.job_object           │    │
│                              │  - windows.wsl2                 │    │
│                              └─────────────────────────────────┘    │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
┌──────────────────────────────▼──────────────────────────────────────┐
│  Plugin Bus (out-of-process gRPC over Unix socket)                  │
│  - scanner plugins   - classifier plugins   - audit sinks           │
│  - agent adapters    - network policy plugins                       │
└─────────────────────────────────────────────────────────────────────┘
```

The Decision Engine remains the only component that says ALLOW/ASK/BLOCK.
Enforcement turns a decision into syscalls. Plugins **only contribute
evidence** (findings, classifications, audit events). Plugins never override
the decision engine.

---

## 3. Policy schema additions (v1)

New top-level sections. All optional. Absent = current MVP behavior.

```yaml
sandbox:
  enabled: false                # opt-in; CLI flag --sandbox=auto|off|<backend>
  backend: auto                 # auto picks best available for OS
  fail_closed: true             # if requested backend missing, BLOCK the run
  allowlist_syscalls: []        # seccomp profile name or inline list
  allowlist_paths_extra: []     # extra paths granted on top of filesystem.*
  egress:
    mode: off                   # off | proxy | block
    proxy_addr: "127.0.0.1:8443"

plugins:
  scanner:    [gitleaks, trufflehog]
  classifier: [semgrep_rules]
  audit_sink: [stdout_jsonl, splunk_hec]
  agent_adapter: [mcp_server]
  network_policy: []

integrations:
  github:
    branch_protection_aware: true
    token_env: GITHUB_TOKEN     # read-only, used only for branch-protection lookup
```

`sandbox.fail_closed: true` is the secure default for any policy that opts in
to a sandbox: if the OS can't provide it, the run is BLOCKED rather than
running un-sandboxed.

---

## 4. Sandbox backends

### 4.1 macOS

**Primary backend:** `sandbox-exec` (Seatbelt) with a generated `.sb` profile.

```
internal/sandbox/macos/seatbelt.go
  - GenerateProfile(policy) -> .sb text
  - Wrap(argv) -> exec.Cmd that runs `sandbox-exec -f profile.sb -- argv...`
```

Profile generator pattern (TinyScheme syntax):

```scheme
(version 1)
(deny default)
(allow process-fork process-exec)
(allow file-read* (subpath "/Users/me/proj"))
(deny  file-read* (subpath "/Users/me/.ssh"))
(deny  file-read* (subpath "/Users/me/.aws"))
(allow file-write* (subpath "/Users/me/proj/src"))
(allow file-write* (subpath "/Users/me/proj/tests"))
(deny  file-write* (subpath "/Users/me/proj/.git/config"))
(deny  network*)                       ; toggled by sandbox.egress
(allow network-outbound (remote tcp "github.com:443"))
```

**Limitations to document loudly:**

- `sandbox-exec` is **deprecated by Apple** in macOS man page (still works on
  macOS 14/15 but no API stability guarantee).
- Modern Apple sandboxing requires the binary to be **signed with an
  entitlement** and use App Sandbox, which is not viable for a developer CLI
  that supervises arbitrary child processes.
- Seatbelt profile errors are silent and hard to debug. Meerkat ships a
  `meerkat sandbox test` subcommand that runs probe commands to validate the
  profile before the agent runs.
- No equivalent of Linux Landlock for unprivileged-process self-restriction
  with stable API.
- Network controls in Seatbelt are coarse. For real egress filtering on
  macOS, layer the Linux-style egress proxy (4.2.4) and force libraries via
  `HTTPS_PROXY`. Recognize that a child process can ignore env vars.

Recommendation in v1 docs: "macOS sandbox enforcement is best-effort.
Consider running Meerkat inside an OrbStack / Lima Linux VM for parity with
the Linux backend on adversarial workloads."

### 4.2 Linux

#### 4.2.1 Landlock (filesystem)

`internal/sandbox/linux/landlock.go` uses the Landlock LSM via direct
syscalls (`landlock_create_ruleset`, `landlock_add_rule`,
`landlock_restrict_self`). Available on kernel ≥ 5.13; v3 ruleset on 6.7.

Behavior:

- Translate `filesystem.allowed_read_paths` → `LANDLOCK_ACCESS_FS_READ_*` rules.
- Translate `allowed_write_paths` → `LANDLOCK_ACCESS_FS_WRITE_FILE` + dir flags.
- `blocked_paths` are simply absent from the ruleset (Landlock is allow-list).
- Apply the ruleset to the supervised process via a `PR_SET_NO_NEW_PRIVS` +
  `landlock_restrict_self` in the child's `PreExec` hook so the Meerkat
  parent is not restricted.

Limitations:

- Landlock does **not** restrict syscalls — pair with seccomp.
- Landlock does **not** cover all FS operations (truncate added late, ioctl
  added in v3). Document supported kernel matrix.
- No network control. Pair with egress proxy.

#### 4.2.2 seccomp (syscalls)

`internal/sandbox/linux/seccomp.go` uses
`github.com/seccomp/libseccomp-golang` (with a pure-Go fallback for static
builds).

Default profile:

- Allow the syscalls used by `git`, `node`, `python`, `cargo`, `go`,
  modern coreutils.
- Deny `ptrace`, `process_vm_readv/writev`, `kexec_*`, `bpf` (unless
  agent profile opts in), `mount`, `pivot_root`, `unshare(CLONE_NEWUSER)`,
  `init_module`, `delete_module`.
- `kill` allowed for self-pgroup only via `SCMP_CMP(arg0, SCMP_CMP_EQ, pgid)`.

Profile naming: `seccomp.default`, `seccomp.builder`, `seccomp.agent`. Users
can extend via `sandbox.allowlist_syscalls`.

#### 4.2.3 Process isolation — namespaces / bubblewrap

Two modes:

1. **`bubblewrap`** (recommended, user-namespace based, no setuid).
   `internal/sandbox/linux/bwrap.go` constructs:

   ```bash
   bwrap \
     --ro-bind / / --dev-bind /dev /dev --proc /proc \
     --tmpfs /tmp \
     --ro-bind ~/.ssh /dev/null      # mask
     --ro-bind ~/.aws /dev/null      # mask
     --bind  $PROJECT $PROJECT \
     --new-session --die-with-parent \
     --unshare-net                   # if egress=block
     -- $CHILD_ARGV
   ```

2. **Direct unshare** when bubblewrap is unavailable: `unshare(CLONE_NEWUSER
   | CLONE_NEWNS | CLONE_NEWNET | CLONE_NEWPID | CLONE_NEWIPC)`. Requires
   `kernel.unprivileged_userns_clone=1`. Document.

Pre-flight check (`meerkat doctor`) reports which Linux backends are
available and what kernel/sysctl conditions are missing.

#### 4.2.4 Egress proxy (network control)

`internal/sandbox/linux/egress/proxy.go` runs a per-run HTTPS-aware forward
proxy on `127.0.0.1:<random>`. Behavior:

- Listens for HTTP `CONNECT` (TLS pass-through). Reads SNI from the TLS
  ClientHello to learn the destination domain without terminating TLS.
- Applies `network.allow_domains` / `block_domains`. Unknown → ASK or BLOCK
  per `network.default`.
- Logs every connection attempt as an audit `network_egress` event.
- Sets `HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY=` in the child env.
- For hard enforcement, combines with `--unshare-net` and a `slirp4netns`
  tunnel that only routes through the proxy.

Plain HTTP requests are also proxied; the proxy enforces Host: header against
the allow list and rejects mismatched SNI/Host pairs to defeat domain
fronting.

Documented gap: a process can still issue raw UDP / ICMP / raw-socket
traffic if not in a network namespace. The default profile pairs the proxy
with `unshare(CLONE_NEWNET)` so the only egress path is the proxy.

### 4.3 Windows

#### 4.3.1 WSL2 backend (recommended for agent workloads)

`internal/sandbox/windows/wsl.go` detects a configured WSL2 distro and
transparently re-execs `meerkat run` inside it with the Linux backend
selected. Project path is bind-mounted via `wsl --cd`.

Result: Windows users get the **full Linux sandbox stack** (Landlock,
seccomp, bubblewrap, egress proxy) for any cross-platform workflow.

#### 4.3.2 AppContainer

`internal/sandbox/windows/appcontainer.go` creates an AppContainer profile
with `CreateAppContainerProfile` + `SetAppContainerInformation`, restricts
capabilities to `internetClient` (toggled by policy), and launches the child
via `CreateProcess` with the AppContainer SID.

Limitations:

- AppContainers restrict access to securable objects (registry, files via
  ACLs); they do **not** sandbox arbitrary process behavior the way
  Landlock+seccomp does on Linux.
- Many developer tools fail inside AppContainer because they touch user
  profile paths the container's SID cannot read. Meerkat ships a curated
  capability set per profile (`builder`, `agent`).
- AppContainer cannot block egress; pair with Windows Firewall rules.

#### 4.3.3 Job Objects

`internal/sandbox/windows/jobobject.go` always wraps the child in a Job
Object. Use for:

- `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` — guarantees no orphans when Meerkat
  exits.
- `JOB_OBJECT_LIMIT_ACTIVE_PROCESS` — caps process count.
- `JOB_OBJECT_LIMIT_BREAKAWAY_OK = false` — prevent escape.
- `UILimits` to block clipboard / global atom abuse from the agent.

This is **always-on** (no opt-in) because it improves baseline safety with
zero policy impact.

#### 4.3.4 Windows Firewall rules

Per-run dynamic outbound firewall rules scoped to the supervised PID via
`netsh advfirewall` or the COM `INetFwPolicy2` interface. Implementation
gotcha: Windows Firewall does not natively scope rules per-PID; we approximate
by binding rules to the AppContainer SID, or by routing the child through a
loopback proxy similar to Linux.

**What is and is not enforced on Windows:**

| Enforcement | Backend | Status |
|------------|---------|--------|
| Process containment | Job Object | Always on |
| Filesystem allow/block | AppContainer ACLs | Opt-in, partial |
| Syscall filtering | none on Windows | Not available |
| Network egress | Firewall + loopback proxy | Opt-in, best-effort |
| Full parity with Linux | WSL2 re-exec | Recommended path |

Honest doc line: "Windows native enforcement is weaker than Linux. For
adversarial agent workloads, run inside WSL2 and let Meerkat use the Linux
backend."

---

## 5. Plugin system

### 5.1 Transport

Plugins are out-of-process gRPC servers over Unix domain sockets (named pipe
on Windows). One plugin = one process = one socket. Lifecycle managed by the
Meerkat core (`internal/plugins/manager.go`):

- Start plugin lazily on first use, with a watchdog and timeout.
- Each plugin runs as a child process of Meerkat under the same UID, with
  the same sandbox profile applied to it where reasonable.
- Plugin crashes never propagate. A failed plugin = failed evidence = the
  decision engine treats it as a soft warning (or BLOCK if the policy marks
  the plugin as critical with `plugins.<kind>[].required: true`).

### 5.2 Plugin manifest

```yaml
plugin:
  name: gitleaks-scanner
  kind: scanner            # scanner|classifier|agent_adapter|network_policy|audit_sink
  version: 1.0.0
  binary: ./meerkat-gitleaks
  protocol: grpc-unix
  required: false
  permissions:             # what core grants the plugin
    read_paths: [.]
    network: none
```

Core launches the plugin with a freshly minted token over the socket; the
plugin must echo the token within 2s or be killed.

### 5.3 Plugin interfaces (proto, abbreviated)

```proto
service Scanner {
  rpc Scan(ScanRequest) returns (stream Finding);
}
message ScanRequest { repeated string paths = 1; bytes policy_blob = 2; }
message Finding { string file = 1; int32 line = 2; string type = 3;
                  string redacted = 4; double confidence = 5; }

service Classifier {
  rpc Classify(ClassifyRequest) returns (Classification);
}
message ClassifyRequest { string command_line = 1; bytes policy_blob = 2; }
message Classification { string risk = 1; repeated string reasons = 2;
                         bool network_likely = 3; }

service AgentAdapter {
  rpc OnToolCall(ToolCall) returns (ToolDecision);
}
message ToolCall { string tool = 1; bytes args_json = 2; string agent_id = 3; }
message ToolDecision { string action = 1; repeated string reasons = 2; }

service NetworkPolicy {
  rpc Evaluate(NetEvalRequest) returns (NetEvalResponse);
}
message NetEvalRequest { repeated string domains = 1; string command_line = 2; }
message NetEvalResponse { bool allowed = 1; bool ask = 2; string reason = 3; }

service AuditSink {
  rpc Emit(stream AuditEvent) returns (Ack);
}
message AuditEvent { string json = 1; }
```

### 5.4 Trust model for plugins

- Plugins are **untrusted code** unless signed by a key in
  `~/.config/meerkat/trusted_plugin_keys`.
- Unsigned plugins prompt the user once with a SHA-256 fingerprint on first
  load and store the trust grant. Treat exactly like SSH known_hosts.
- A plugin cannot change the decision; it can only contribute evidence and
  request `ASK`/`BLOCK` via its return value. The decision engine combines
  evidence via documented merge rules:

  | Source | Effect |
  |--------|--------|
  | Any scanner finding with `confidence >= 0.7` and `secrets.scan_before_*` true | Force BLOCK on commit/push |
  | Classifier plugin returning higher risk than core | Adopted (raise risk only, never lower) |
  | Network plugin returning BLOCK | Adopted |
  | Audit sink failure | Logged locally; never blocks the run |
  | Agent adapter returning DENY | Treated like user denying approval |

  **Plugins can raise risk; only the core decision engine can lower it.**

---

## 6. Integrations

All integrations ship as plugins. None are required.

### 6.1 gitleaks

`plugins/gitleaks/` wraps `gitleaks detect --no-banner --report-format=json`
into the `Scanner` interface. Findings stream to the core; redaction happens
in the wrapper before crossing the gRPC boundary.

### 6.2 trufflehog

`plugins/trufflehog/` wraps `trufflehog filesystem . --json --no-update`.
Same interface; verified=true findings get `confidence: 0.95`.

### 6.3 detect-secrets

`plugins/detect-secrets/` wraps `detect-secrets scan --baseline=.secrets.baseline`.
Honors `.secrets.baseline` so existing waivers are respected.

### 6.4 semgrep

`plugins/semgrep/` runs `semgrep --config=auto --json` and exposes results as
a **Classifier** plugin that can raise risk on auth/security file edits
(threat T12 in the threat model). Rule packs documented for: hardcoded
credentials, dangerous deserialization, command injection sinks, weak crypto,
auth-bypass patterns.

### 6.5 GitHub branch-protection awareness

`internal/integrations/github/` reads
`GET /repos/{owner}/{repo}/branches/{branch}/protection` when
`integrations.github.branch_protection_aware: true` and a read-only token is
configured. The classifier merges remote-side protected branches with the
local `git.protected_branches`. Cache TTL: 1 h. Result: a push to a remote
branch protected on GitHub gets BLOCKED even if the local policy did not
list it.

Fails open with a warning if the token is missing or the API is unreachable;
never fails open silently.

### 6.6 MCP approval server

`cmd/meerkat-mcp/` exposes Meerkat over the **Model Context Protocol** as a
small server with three tools:

| MCP tool | Behavior |
|---------|----------|
| `meerkat.explain` | Run the classifier + decision engine, return JSON decision |
| `meerkat.approve` | Surface an approval prompt to the user; return ALLOW/DENY |
| `meerkat.scan`    | Run scanner plugins on a path; return findings |

This lets MCP-aware agents (Claude, Codex, Cursor's MCP) request approval
*before* invoking a tool, instead of after. Server enforces the same policy
and emits the same audit events.

### 6.7 VS Code / Cursor extension

`extensions/vscode/` ships as a VSIX with three surfaces:

- Status-bar item: shows current policy, keep-awake state, last decision.
- Approval webview: replaces the terminal prompt when the editor is open;
  shows risk, reasons, diff of files about to change, network destinations.
- Inline lens above `git push`, `git commit`, and shell command palettes:
  "Meerkat: would block — push to protected branch `main`".

The extension talks to `meerkat` over its MCP server (6.6), so VS Code and
Cursor reuse the same code path as headless runs. No second policy engine.

---

## 7. CLI surface additions (v1)

```text
meerkat run --sandbox=auto -- <cmd>           # opt-in isolation
meerkat run --sandbox=off  -- <cmd>           # explicit no-sandbox (loud audit event)
meerkat run --sandbox=landlock,seccomp -- ... # composite, Linux

meerkat sandbox doctor                        # which backends are available
meerkat sandbox test                          # run a probe set, validate profile
meerkat sandbox profile show                  # print generated .sb / seccomp / landlock spec

meerkat plugin list
meerkat plugin install <repo-or-path>
meerkat plugin trust <fingerprint>
meerkat plugin remove <name>

meerkat mcp                                   # run the MCP approval server
meerkat egress proxy                          # run the egress proxy standalone (debug)
```

`--sandbox=auto` selects the best available backend for the current OS and
policy intent; refuses to run if `sandbox.fail_closed: true` and nothing is
available.

---

## 8. Audit additions (v1)

New event types:

- `sandbox_started` — backend, profile hash
- `sandbox_denied`  — what the backend blocked (file, syscall, domain)
- `network_egress`  — host, port, allowed/blocked, plugin used
- `plugin_loaded`   — name, version, fingerprint, trust state
- `plugin_finding`  — kind, summary (full finding redacted by plugin)
- `mcp_tool_call`   — tool, agent_id, decision

`sandbox_denied` events are the new gold standard for forensics — they
capture the syscalls and paths the agent **wanted** to touch but was
prevented from touching. These should never be missing in a real incident
post-mortem.

---

## 9. Build, distribution, supply chain

- Cosign-signed release binaries; SLSA-3 provenance.
- Reproducible builds via `-trimpath -ldflags="-s -w -buildid="` and pinned
  toolchain in `go.work`.
- Homebrew tap (`brew install ujjwalredd/meerkat/meerkat`).
- One-line installer that verifies signature before unpack
  (`curl -sSf https://meerkat.dev/install.sh | sh` — script also verifies).
- Plugins published via a simple manifest registry; same Cosign verification.
- No telemetry. Update check is opt-in (`meerkat update --check`) and uses a
  static GitHub releases URL with no identifying headers.

---

## 10. Default vs. advanced experience

| User | Default behavior |
|------|------------------|
| `meerkat init` then `meerkat run -- npm test` | No sandbox, no plugins. Identical to MVP. |
| `meerkat init --profile=agent`               | Enables `sandbox.enabled: true`, `backend: auto`, `fail_closed: true`. Pulls strict policy. |
| `meerkat init --profile=strict`              | Enables sandbox + egress proxy + GitHub branch-protection awareness. No auto-approve. |

**Advanced isolation is opt-in until each backend has shipped two stable
releases and passed a third-party audit.** During the beta of each backend,
`meerkat doctor` prints `[beta]` next to it and the CLI requires
`--sandbox=<name>` to be named explicitly — `auto` will not select a beta
backend by default.

---

## 11. Implementation order (v0.3 → v1.0)

1. **v0.3** — Shell-proxy mode, MCP approval server, agent adapter interface.
   Plugins for `gitleaks`, `trufflehog`, `detect-secrets`. VS Code extension
   alpha. **No sandbox backends yet.**
2. **v0.4** — Linux Landlock + seccomp backends behind `--sandbox=` flag.
   Linux egress proxy + `slirp4netns` integration. Bubblewrap backend. macOS
   Seatbelt backend (best-effort). Windows Job Object always-on. WSL2 re-exec.
3. **v0.5** — Windows AppContainer + Firewall rule integration. GitHub
   branch-protection-aware classifier. Semgrep classifier plugin.
4. **v0.6** — Plugin signing, plugin registry, trust UX.
5. **v0.7** — Reproducible builds, Cosign-signed releases, SLSA provenance.
6. **v1.0** — Stable policy schema v2 with sandbox + plugins; third-party
   security audit completed; deprecation policy in place; first LTS line.

---

## 12. Non-goals (still)

- Replacing antivirus / EDR.
- Kernel-mode rootkit-resistant tamper protection of the Meerkat binary itself.
- Hypervisor-level isolation (use a VM).
- Defeating a skilled attacker who already runs as your UID.
- Sandboxing the developer. Meerkat sandboxes what the developer (or their
  agent) launches; the developer remains the trust root.

---

## 13. Open questions

- Should the plugin bus support WASM plugins for portability? (Pro: no
  per-OS plugin binaries. Con: weaker permission model than separate
  processes.)
- Should the MCP server expose `meerkat.diff_preview` so the agent can show
  the user the diff before requesting approval, inline?
- macOS: invest in Endpoint Security Framework (requires Apple-signed
  System Extension entitlement, hard distribution story) or commit to "use
  Linux backend via Lima/OrbStack VM"?
- Windows: ship a kernel-mode minifilter for filesystem enforcement, or
  remain WSL2-first?

Decisions on these belong in `docs/decisions/` ADRs before code lands.
