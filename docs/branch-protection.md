# Branch and tag protection

These settings keep Meerkat releases predictable. They are recommended for the
upstream repository and for forks that publish binaries.

## Branch rules

In GitHub, open **Settings > Rules > Rulesets** and create a branch ruleset for:

- `main`
- `master`
- `production`

Recommended settings:

- Require a pull request before merging.
- Require status checks to pass.
- Require the CI jobs from `.github/workflows/ci.yml`: test on Ubuntu, test on
  macOS, Windows build, and secret scan.
- Block force pushes.
- Block branch deletion.
- Require conversation resolution.
- Restrict bypass permissions to repository admins or release maintainers.

If you use classic branch protection instead of rulesets, apply the same
requirements under **Settings > Branches**.

## Tag rules

Create a tag ruleset for:

- `v*.*.*`

Recommended settings:

- Block tag deletion.
- Block tag updates after creation.
- Restrict tag creation to release maintainers.

Do not retag a published version. If a tag is wrong, create a new patch release
such as `v0.4.2` and document the bad tag in `CHANGELOG.md`.

## Release workflow checks

The release workflow runs on tags matching `v*.*.*`. Before pushing a tag,
run:

```bash
go test ./...
go test -race ./...
make lint
bash -n scripts/install.sh
npm --prefix npm test
make release-local
```

After GitHub publishes the release, verify the release assets:

```bash
meerkat doctor --release
```

See [`docs/release.md`](release.md) for the full release process.
