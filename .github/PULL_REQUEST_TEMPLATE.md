## Summary

<!-- What changes, in one paragraph. -->

## Type

- [ ] Bug fix
- [ ] New feature
- [ ] Classifier / decision rule change
- [ ] Secret pattern change
- [ ] Docs only
- [ ] Refactor / chore

## Security impact

<!-- Required for any change that touches:
     - internal/commandpolicy, internal/decision, internal/scanner,
       internal/filesystem, internal/gitguard, internal/networkpolicy,
       internal/awake, config defaults, or example policies.
     Reference docs/threat-model.md. -->

- Does this **loosen** any default? If yes, justify against the threat model.
- Does this introduce new network egress, new file writes, or new shell-out?
- Does this add any path where an LLM influences a security decision? (Must be no.)

## Tests

- [ ] `go test ./...` passes locally
- [ ] `go vet ./...` clean
- [ ] `gofmt -l .` clean
- [ ] Added/updated tests for the change

## Docs

- [ ] `README.md` updated if user-visible
- [ ] `docs/policy.md` updated if policy schema changed
- [ ] `docs/threat-model.md` updated if threat surface changed
- [ ] `CHANGELOG.md` entry under `Unreleased`

## Checklist

- [ ] One logical change per PR
- [ ] No telemetry, no cloud calls, no LLM in security decisions
- [ ] Default policy still strict
