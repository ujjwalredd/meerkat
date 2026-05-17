# Security Policy

## Reporting a vulnerability

**Do not open public GitHub issues for security problems.**

Report privately via GitHub Security Advisories:
**https://github.com/ujjwalredd/meerkat/security/advisories/new**

Include:

- A description of the issue and its impact.
- A minimal reproducer (policy file + command line).
- The Meerkat version (`meerkat version`).
- Your OS / architecture.

We aim to:

- Acknowledge within **5 business days**.
- Ship a fix or coordinated mitigation within **30 days** of a confirmed report.
- Credit reporters in `CHANGELOG.md` unless they prefer anonymity.

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | Yes       |
| < 0.1   | No        |

## Threat model

The full threat model is in [`docs/threat-model.md`](docs/threat-model.md).
A condensed summary:

### Assumptions

- Meerkat runs as the invoking user, never as root/admin.
- The host operating system and base toolchain (`git`, package managers,
  shell) are not compromised.
- Commands run **outside** `meerkat run` are not supervised.
- The agent being wrapped does not have direct kernel access.

### What we protect against

Prompt-injected and misbehaving AI coding agents reading secrets, writing
secrets into source, exfiltrating over the network, pushing to protected
branches, force-pushing, escalating privilege, recursive force-removal,
out-of-scope writes, symlink escape, and shell injection.

### What we do **not** protect against

- Kernel exploits, malware, EDR replacement.
- Compromised toolchain or supply-chain attacks already on disk.
- Packet-level network egress (MVP is command/policy-based).
- A malicious binary launched by an allowed command (Meerkat checks what you
  launch, not every syscall the launched process makes).
- A skilled attacker running as your UID — they can defeat any user-space
  wrapper, including this one.

For high-risk workloads, also run Meerkat inside a container or VM.

## Hardening recommendations

- Run Meerkat as your normal user. Never with `sudo`.
- Keep `mode.default_action: ask` or `block`.
- Always keep `block_outside_project: true`.
- Enable `secrets.scan_before_commit` and `scan_before_push`.
- Make `meerkat.yml` read-only to the agent's UID where practical.
- Combine with a container or VM for untrusted agent workloads.
