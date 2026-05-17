# Meerkat Threat Model

> Version: 0.1 (MVP). Last updated: 2026-05-17.

---

## 1. Overview

Meerkat is a user-space CLI security wrapper that supervises commands run by
developers and AI coding agents. It applies a deterministic, rule-based policy
to every command, classifies the action into `ALLOW` / `ASK` / `BLOCK`, and
logs every decision.

This document describes:

- What Meerkat is built to protect
- What it is **not** built to protect
- The trust boundaries it crosses
- The adversaries it assumes
- The security controls it implements today
- The honest limitations of an MVP user-space tool
- Recommended safe deployment patterns
- The hardening roadmap

Meerkat MVP is a hardened **policy runner**, not a sandbox. It cannot replace
kernel-enforced isolation. Strong isolation in adversarial environments still
requires containers or VMs. Meerkat raises the floor; it does not put a
ceiling on what a determined attacker can do once a malicious binary executes.

---

## 2. Assets protected

In order of severity if compromised:

| Asset | Why it matters |
|-------|----------------|
| SSH private keys (`~/.ssh/id_*`) | Account takeover across every git/SSH host |
| Cloud credentials (`~/.aws`, `~/.config/gcloud`, kube configs) | Full cloud blast radius |
| API keys (OpenAI, Anthropic, GitHub PAT, Stripe, Slack, internal) | Billing fraud, data exfiltration, lateral movement |
| Browser/keychain secrets | Identity theft |
| `.env`, `secrets/`, `.git-credentials` | Same as above, harder to rotate |
| Source code & IP | Disclosure / poisoning |
| Git remotes & protected branches | Supply-chain compromise of downstream users |
| CI/CD configuration (`.github/workflows`, `.gitlab-ci.yml`) | Secret extraction via privileged runners |
| Lockfiles, vendored deps | Long-lived backdoor surface |
| Auth/security-related code paths | Privilege bypass in production |
| Developer machine integrity | Pivot to corporate network |
| Developer identity (signing keys, commit author) | Forged commits |

---

## 3. Trust boundaries

Meerkat sits between the developer (or the AI agent the developer launched)
and the operating system. The trust boundaries crossed:

```
        ┌──────────────────────────────┐
        │   Human developer            │  ← root of trust
        └────────────┬─────────────────┘
                     │ invokes
                     ▼
        ┌──────────────────────────────┐
        │   meerkat (this tool)        │  ← policy engine
        │   - parses policy            │
        │   - classifies commands      │
        │   - logs decisions           │
        └────────────┬─────────────────┘
                     │ exec
        ┌────────────▼─────────────────┐
        │   Supervised process         │  ← UNTRUSTED in agent runs
        │   (agent / build / shell)    │
        └────────────┬─────────────────┘
                     │ syscalls
        ┌────────────▼─────────────────┐
        │   Operating system           │  ← assumed not compromised
        └──────────────────────────────┘
                     │
        ┌────────────▼─────────────────┐
        │   Network / git remotes /    │  ← partially untrusted
        │   package registries          │
        └──────────────────────────────┘
```

Boundaries enforced by Meerkat:

- **policy → supervised process**: command must pass classifier
- **supervised process → filesystem**: post-run `git status` reconciliation;
  pre-run path scope checks for explicit reads of blocked paths
- **supervised process → git remote**: secret scan + branch check before commit/push
- **supervised process → network**: command/policy-based domain allow/block (MVP)

Boundaries **not** enforced by Meerkat (today):

- **supervised process → kernel syscalls** (no seccomp/Landlock/Seatbelt yet)
- **supervised process → arbitrary fork+exec inside its sandbox** (an allowed
  binary can spawn anything its UID permits)
- **packet-level network egress** (no firewall, no DNS sinkhole)

---

## 4. Threat actors

| Actor | Capability | Motivation |
|-------|-----------|-----------|
| **Misbehaving AI agent** | Runs shell commands chosen by an LLM; non-malicious but non-deterministic | Hallucination, over-eager autonomy |
| **Prompt-injected AI agent** | Same shell access; instructions come from attacker-controlled content (page, README, issue, web search result, file the agent read) | Steered by remote attacker to exfiltrate or sabotage |
| **Malicious dependency** | Runs lifecycle scripts during `npm install` / `pip install` / `cargo build` | Credential theft, crypto mining, backdoor seeding |
| **Compromised dev tool** | An IDE plugin or CLI tool the dev installed | Persistence, lateral movement |
| **Curious or rushed developer** | Wants agent convenience; clicks `y` without reading | Mistake-driven exfiltration / unsafe push |
| **External attacker, post-credential-theft** | Has API key, wants more | Same as agent threats but persistent |

Meerkat is most useful against **non-malicious-but-non-trustworthy** agents
and **prompt-injected** agents. It is least useful against an attacker who
has already achieved code execution as the developer and bypasses Meerkat
entirely.

---

## 5. In-scope threats

Each row below is a concrete attack path Meerkat is designed to mitigate today.

| # | Threat | Mitigation |
|---|--------|-----------|
| T1 | **Prompt injection → shell command.** External content tells the agent to run an attacker-controlled command. | Deterministic classifier on every command. `curl`, `wget`, `ssh`, `scp`, `nc` blocked by default. Unknown commands ASK or BLOCK per `mode.default_action`. |
| T2 | **Agent reads `.env`.** | `filesystem.blocked_paths` includes `./.env` and `./.env.*`. Command lines that reference blocked paths trigger BLOCK. |
| T3 | **Agent reads `~/.ssh`.** | `~/.ssh`, `~/.aws`, `~/.config`, `~/Library/Application Support` blocked by default. Symlinks resolved via `EvalSymlinks` before scope check. |
| T4 | **Agent writes API key into source code.** | Secret scan runs before `git commit` and `git push` (`secrets.scan_before_commit/push`). Findings BLOCK the operation. Built-in detectors for AWS, GitHub, OpenAI, Anthropic, Slack, Stripe, JWT, DB URLs, private keys, generic `key="…"` assignments. |
| T5 | **Agent installs malicious dependency.** | `npm install`, `pip install`, `pnpm add`, `yarn add`, `cargo add`, `go get` are heuristically MEDIUM and policy-listed `require_approval` → ASK with reasons before execution. |
| T6 | **Agent modifies GitHub Actions workflow.** | `.github/workflows/*` is not under `allowed_write_paths` in default policy → post-run scope check flags as `policy_violation`. Tighten by excluding from `allowed_write_paths` and listing in `blocked_paths`. |
| T7 | **Agent pushes directly to `main`.** | Classifier parses `git push <remote> <branch>` and matches against `git.protected_branches`. Default: BLOCK on push to `main`, `master`, `production`. |
| T8 | **Agent sends code to unknown URL.** | URLs are extracted from the command line. Domain allow/block evaluated against `network.allow_domains` / `block_domains`. Unknown → ASK (when `require_approval_for_unknown_domains: true`) or BLOCK (when `network.default: block`). |
| T9 | **Agent deletes project files.** | `rm -rf` recursive force patterns BLOCK. Single-file deletes still allowed under scope (post-run `git status` flags out-of-scope removals). |
| T10 | **Agent modifies files outside project scope.** | Project root is determined and resolved. Post-run `git status` is compared against `filesystem.allowed_write_paths`; any out-of-scope change is logged as `policy_violation`. Symlink escape: real-path resolution before comparison. |
| T11 | **Agent changes package scripts to run malicious commands.** | `package.json`, lockfiles, `pyproject.toml`, `Cargo.toml` are sensitive. Default `allowed_write_paths` does **not** include them; modifications appear as out-of-scope post-run. Tighten further by listing them in `blocked_paths` so any write attempt is flagged. |
| T12 | **Agent modifies auth/security-related code without review.** | Same scope mechanism: place `./src/auth`, `./src/security`, `./middleware`, `./internal/auth` etc. under `blocked_paths` (or simply omit from `allowed_write_paths`). Changes surface in post-run audit. |
| T13 | **Privilege escalation.** | `sudo`, `su`, `doas` BLOCK by default and by heuristic. Meerkat itself should never be run with elevated privileges. |
| T14 | **Force push.** | `--force` / `--force-with-lease` / shorthand `-f` parsed and BLOCKED. |
| T15 | **Recursive `chmod` / `chown`.** | Pattern-matched and BLOCKED. |
| T16 | **Shell command injection via agent-generated string.** | Meerkat runs commands via `exec.Command(argv[0], argv[1:]...)`. No shell is invoked. Argument boundaries are preserved. |
| T17 | **TOCTOU between approval and execution.** | Approval is in-process and synchronous. There is no window for the agent to mutate the command between approval and `exec`. |

---

## 6. Out-of-scope threats

Meerkat MVP does **not** mitigate these. Documented so users do not assume protection.

| # | Threat | Why out of scope |
|---|--------|------------------|
| O1 | Kernel exploits / rootkits | Requires OS-level defense (SELinux, EDR, hypervisor). |
| O2 | Side-channel exfiltration (DNS tunneling from an allowed process, file timestamps, image steganography in commit blobs) | MVP inspects commands, not binary behavior. |
| O3 | A malicious binary that an **allowed** command launches | Meerkat checks `npm test`; it does not sandbox the resulting Node process. |
| O4 | Supply-chain compromise of a transitive dependency that has already been installed | Detection requires SBOM analysis, lockfile diffing, reproducible builds — out of scope for MVP. |
| O5 | Compromised git, package manager, or shell on disk | Meerkat trusts the toolchain it shells out to. |
| O6 | Packet-level firewall enforcement | MVP is command/policy-based. A future egress proxy will close this. |
| O7 | Process running **outside** `meerkat run` | The agent must be launched under Meerkat to be supervised. |
| O8 | Race between symlink resolution and use (TOCTOU window) | A determined attacker can swap a target between `EvalSymlinks` and `open`. |
| O9 | Resource exhaustion / fork bombs / CPU and disk DoS | Out of scope; use OS quotas. |
| O10 | Cryptographic supply-chain (compromised signing keys, malicious release binaries) | Use signed releases (Cosign/Sigstore) — Meerkat itself plans this for v1.0. |
| O11 | Hardware key extraction, cold-boot attacks | Physical security is the OS / hardware owner's job. |
| O12 | An attacker who already has shell as your UID | They can disable or modify Meerkat. Meerkat is not a rootkit. |

Meerkat **does not** claim to replace antivirus, EDR, host-based IDS, or a
container.

---

## 7. Security controls

| Layer | Control | Status |
|-------|---------|--------|
| **Classifier** | Deterministic rule-based, no LLM in decision path | Implemented |
| **Policy** | YAML; strict defaults; `version: 1` required; validated on load | Implemented |
| **Command exec** | `exec.Command(argv...)` — no shell interpolation | Implemented |
| **Path scope** | Real-path resolution via `EvalSymlinks`; project-root check; glob support | Implemented |
| **Pre-run** | Block list, heuristic high-risk match, protected-branch detection | Implemented |
| **Pre-commit/push** | Built-in secret scan; BLOCK on any finding | Implemented |
| **Post-run** | `git status` reconciled against `allowed_write_paths`; out-of-scope changes logged as `policy_violation` | Implemented |
| **Network** | Domain extraction from command; allow/block lists; unknown → ASK | Implemented (command-level only) |
| **Approval prompt** | Timeout-bounded; `default_on_timeout: deny`; no auto-allow | Implemented |
| **Keep-awake** | `caffeinate` (macOS), `systemd-inhibit` (Linux); max duration enforced; always stopped on exit | Implemented |
| **Audit log** | JSONL, append-only, per-run file; secrets redacted; command output **not** stored by default | Implemented |
| **Signal handling** | Forwards Ctrl-C / SIGTERM to child's process group; cleans up keep-awake | Implemented |
| **Privilege model** | Meerkat refuses to escalate; runs as invoking user | Documented; user must comply |
| **Unknown command policy** | Configurable: `ask` (default), `allow`, `block`. Never silent-allow. | Implemented |

---

## 8. Known limitations

Stated bluntly because trust is built on honesty:

1. **User-space only.** Meerkat does not use seccomp, Landlock, Seatbelt,
   AppContainer, eBPF, or any kernel-level isolation in MVP. **A user-space
   policy runner is not equivalent to a VM or a kernel-enforced sandbox.**
2. **A malicious binary launched by an allowed command can do anything that
   binary is allowed to do.** Meerkat checks **what you launch**, not every
   syscall the launched process subsequently makes. `npm test` may execute
   arbitrary JavaScript with full UID privileges.
3. **Network controls are command-line based** in MVP. They detect `curl`,
   `wget`, `npm install`, etc. They do **not** stop a process that opens a
   raw socket.
4. **Secret scanner is regex-based.** False negatives are possible.
   High-entropy obfuscated secrets may evade detection.
5. **TOCTOU window** between symlink resolution and file open.
6. **Windows keep-awake** is not implemented. Meerkat says so honestly.
7. **Anything launched outside `meerkat run`** is not supervised.
8. **The agent could disable Meerkat** by unsetting its wrapper, editing
   `meerkat.yml`, or running a child shell directly. Meerkat assumes the
   developer launches the agent under Meerkat and does not include in-process
   tamper-proofing.
9. **No integrity protection on the policy file** beyond filesystem
   permissions. If the agent can write to `meerkat.yml`, the agent can
   loosen the policy.
10. **No SBOM, reproducible-build, or signed-release verification** of the
    binaries `meerkat run` invokes.

---

## 9. Recommended safe usage

Practical recipes that materially increase real-world safety:

### For low-risk vibe coding

- `meerkat init`, accept defaults.
- Run agents under `meerkat run -- <agent>`.
- Keep `mode.default_action: ask`.
- Review the audit log occasionally (`./.meerkat/logs/`).

### For untrusted agents / external content

- Use [`examples/strict/meerkat.yml`](../examples/strict/meerkat.yml):
  `default_action: block`, no auto-approve, no network egress, deny external paths.
- Make `meerkat.yml` **read-only** to the agent's UID:
  `chmod 0400 meerkat.yml`, owned by a different user if possible.
- Run inside a **Docker container** with `--read-only`, `--network=none`
  (or a strict egress proxy), `--cap-drop=ALL`, `--security-opt=no-new-privileges`,
  and mount only the project directory.
- Or run inside a **VM** (multipass, Lima, OrbStack, UTM) with snapshots.
- Use a **dedicated cloud account** with scoped IAM for any credentials the
  agent might need. Never expose admin keys.
- Disable shell history persistence inside the sandbox.

### For maintainers / shared dev machines

- Add `meerkat run` to project Makefiles and onboarding docs.
- Commit `meerkat.yml` to the repo so the policy is reviewed in PRs.
- Pin protected branches in `git.protected_branches`.
- Enable `secrets.scan_before_commit` and `scan_before_push`.
- Run `meerkat scan` in CI as a defense in depth alongside `gitleaks`.

### Universal hygiene

- **Never run `meerkat` with `sudo` or admin.**
- **Never commit `meerkat.yml` overrides that loosen defaults** without a
  documented threat-model rationale.
- Rotate credentials immediately if an agent run flags a `secret_detected`
  event in the audit log.

---

## 10. Future hardening

Roadmap of controls that close known gaps. Tracked in [README.md](../README.md#roadmap).

| Version | Control | Closes |
|---------|---------|--------|
| v0.2 | External scanner integrations (`gitleaks`, `trufflehog`, `detect-secrets`) | Reduces T4 false-negative rate |
| v0.2 | Better install-script detection (pre-flight `package.json` `scripts` analysis) | Strengthens T5 |
| v0.3 | **Shell-proxy mode** — replace the agent's shell with a Meerkat-supervised shim so every inner command is classified | Closes O3 partially (inner commands become visible) |
| v0.3 | **MCP approval server** — agent calls `meerkat.approve(tool, args)` before executing each tool | Native integration with MCP-aware agents |
| v0.3 | Tool-call adapter for structured agent SDKs (Claude, OpenAI function calls) | Per-tool approval, not per-process |
| v0.3 | IDE / VS Code / Cursor extension for inline approval UX | Reduces approval-fatigue clicks |
| v0.4 | **Linux Landlock + seccomp profile** generation per policy | Closes O3, O2 partially |
| v0.4 | **macOS Seatbelt profile** generation per policy | Same on macOS |
| v0.4 | **Windows AppContainer / Job Object** isolation | Same on Windows |
| v0.4 | **Egress proxy** with TLS SNI inspection + domain allow-list at packet level | Closes O6 |
| v0.4 | Policy-file integrity (signature or hash pinning) | Closes part of (9) |
| v1.0 | **Signed releases** (Cosign / Sigstore) | Closes O10 for Meerkat itself |
| v1.0 | Stable policy schema; third-party security audit | Reduces design risk |
| v1.0 | Plugin system for custom classifiers and scanners | Extensibility without forking |

---

## Honest closing note

> A determined, knowledgeable attacker with code execution as your UID can
> defeat any user-space wrapper, including this one. Meerkat is designed to
> make accidental and prompt-injected harm **much** harder, to surface every
> decision in an audit log, and to fail closed by default. It is not designed
> to defeat a skilled adversary running as you. For that, use isolation that
> the kernel enforces.

Report vulnerabilities privately per [`../SECURITY.md`](../SECURITY.md).
