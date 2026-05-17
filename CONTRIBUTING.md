# Contributing

Thanks for your interest in MeerKat.

## Ground rules

- **Security decisions stay deterministic.** No LLM in the decision path.
- **Default policy stays strict.** Loosening defaults requires a documented
  threat-model rationale.
- **No silent network egress.** New features that touch the network must be
  opt-in.
- **No telemetry.** MeerKat is offline-first.
- **Honest documentation.** Do not describe capabilities we don't have.

## Dev loop

```bash
go build ./...
go test ./...
go vet ./...
```

## Adding a classifier rule

1. Add the regex/heuristic in `internal/commandpolicy/classify.go`.
2. Add positive and negative test cases in `classify_test.go`.
3. If the rule affects decisions, add a case in `internal/decision/engine_test.go`.

## Adding a secret pattern

1. Add the rule in `internal/scanner/secrets.go`.
2. Add a detection test and a redaction test in `secrets_test.go`.
3. Confirm the pattern does not fire on the docs in this repo.

## Pull requests

- One logical change per PR.
- Include tests.
- Update `CHANGELOG.md` under `Unreleased`.
- For security-relevant changes, note the threat addressed.

## Releases

Tag `vX.Y.Z`. CI builds binaries for Linux, macOS, Windows. Sign releases.
