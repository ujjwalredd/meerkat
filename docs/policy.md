# Policy reference (`meerkat.yml`)

```yaml
version: 1                              # required, currently 1

project:
  name: my-project
  root: .                               # project root (paths resolved relative to this)

mode:
  default_action: ask                   # ask | allow | block (for unknown commands)
  auto_approve_safe_actions: true       # auto-ALLOW low-risk matches
  deny_out_of_scope: true               # treat writes outside allowed_write_paths as violation
  dry_run: false                        # global dry-run override

awake:
  enabled: true
  mode: while_command_running           # disabled | while_command_running | duration
  max_duration_minutes: 180

filesystem:
  allowed_read_paths: [.]
  allowed_write_paths: [./src, ./tests, ./docs]
  blocked_paths:
    - ./.env
    - ./.env.*
    - ~/.ssh
    - ~/.aws
  block_outside_project: true
  allow_external_paths: []              # explicit exceptions to block_outside_project

commands:
  auto_approve: [npm test, go test ./..., pytest, git status, git diff]
  require_approval: [npm install, pip install, git commit, git push, docker build]
  block: [sudo, su, curl, wget, ssh, scp, nc, "rm -rf /", "chmod -R 777"]

network:
  default: block                        # block | allow | ask
  allow_domains: [github.com, registry.npmjs.org, pypi.org]
  block_domains: [pastebin.com, webhook.site]
  require_approval_for_unknown_domains: true

git:
  protected_branches: [main, master, production]
  block_push_to_protected_branches: true
  require_clean_tests_before_commit: false
  require_secret_scan_before_commit: true
  require_secret_scan_before_push: true
  block_force_push: true

secrets:
  enabled: true
  scan_before_commit: true
  scan_before_push: true
  scan_patterns: [aws_access_key, github_token, openai_api_key, private_key, jwt]
  ignore_paths: [./node_modules, ./.git, ./dist, ./build]
  max_file_bytes: 1048576

audit:
  enabled: true
  log_dir: ./.meerkat/logs
  format: jsonl
  include_command_output: false
  redact_secrets: true

approval:
  prompt_style: compact
  timeout_seconds: 120
  default_on_timeout: deny              # deny | allow
  allow_session_approval: true
  allow_one_time_approval: true

# v0.3+ optional sections. Omit any to keep MVP behavior.

sandbox:
  enabled: false                        # opt-in
  backend: auto                         # auto|off|seatbelt|bwrap|landlock|
                                        # seccomp|jobobject|appcontainer|wsl2
  fail_closed: true                     # BLOCK run if backend unavailable
  allowlist_syscalls: []                # seccomp inline list (planned v0.5)
  allowlist_paths_extra: []             # extra granted paths
  egress:
    mode: off                           # off | proxy | block
    proxy_addr: "127.0.0.1:8443"        # ignored when mode=off

plugins:
  scanner:    [gitleaks, trufflehog]    # auto-activate if on PATH
  classifier: []
  audit_sink: []
  agent_adapter: []
  network_policy: []

integrations:
  github:
    branch_protection_aware: true       # query api.github.com (1h cache)
    token_env: GITHUB_TOKEN             # read-only token env var
```

## Pattern matching

`commands.*` patterns are matched as **prefix on whole tokens**: `npm test`
matches `npm test --watch` but not `npm testfoo`. Single-token patterns like
`sudo` use word-boundary regex.

`filesystem.*` patterns support `filepath.Match` globs (`*`, `?`) and bare
paths (directory prefix match). Symlinks are resolved to their real paths
before comparison; symlinks that point outside the project are out-of-scope.

## Order of evaluation

1. Explicit `commands.block` → BLOCK
2. Heuristic high-risk (sudo, rm -rf, curl, push to protected branch) → BLOCK
3. `commands.auto_approve` + `auto_approve_safe_actions` → ALLOW
4. `commands.require_approval` → ASK
5. Heuristic medium-risk (install, commit) → ASK
6. Low-risk match → ALLOW (if `auto_approve_safe_actions`)
7. Unknown → `mode.default_action` (ask | allow | block)
