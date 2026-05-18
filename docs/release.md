# Release process

Meerkat releases are tag-driven. Tags use the Go module format:
`vMAJOR.MINOR.PATCH`.

## Checklist

1. Confirm branch and tag protection match
   [`docs/branch-protection.md`](branch-protection.md).
2. Update versions:
   - `cmd/meerkat/main.go`
   - `npm/package.json`
   - README and install docs examples
   - `CHANGELOG.md`
3. Run local verification:

   ```bash
   go test ./...
   go test -race ./...
   make lint
   bash -n scripts/install.sh
   npm --prefix npm test
   make release-local
   ```

4. Commit and tag:

   ```bash
   git add .
   git commit -m "Release vX.Y.Z"
   git tag -a vX.Y.Z -m "vX.Y.Z"
   git push origin main
   git push origin vX.Y.Z
   ```

5. GitHub Actions creates the release from the tag.
6. Verify the published release:

   ```bash
   meerkat doctor --release
   ```

   For forks, set `MEERKAT_REPO=owner/name` before running the doctor.

If a published tag is wrong, do not overwrite it. Create the next patch tag
and document the mistake in `CHANGELOG.md`.

## What CI publishes

The `release` workflow builds:

- `meerkat-darwin-amd64`
- `meerkat-darwin-arm64`
- `meerkat-linux-amd64`
- `meerkat-linux-arm64`
- `meerkat-windows-amd64.exe`
- `checksums.txt`
- `checksums.txt.sig`
- `checksums.txt.pem`
- `sbom.spdx.json`

The workflow also creates GitHub artifact attestations for the release files.

## Verify a download

Checksum verification:

```bash
sha256sum -c checksums.txt
```

On macOS:

```bash
shasum -a 256 -c checksums.txt
```

Keyless signature verification with Cosign:

```bash
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/ujjwalredd/meerkat/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```

GitHub artifact attestation verification:

```bash
gh attestation verify meerkat-linux-amd64 \
  --repo ujjwalredd/meerkat
```

## Installer behavior

`scripts/install.sh` resolves the latest GitHub Release, downloads the matching
asset, and verifies it against `checksums.txt` when available.

Set stricter install behavior:

```bash
MEERKAT_REQUIRE_CHECKSUM=1 \
MEERKAT_INSTALL_NO_GO_FALLBACK=1 \
curl -fsSL https://raw.githubusercontent.com/ujjwalredd/meerkat/main/scripts/install.sh | bash
```

## npm publish

Publish after the GitHub Release exists, because the npm wrapper downloads
the binary from GitHub Releases:

```bash
cd npm
npm test
npm publish --provenance
```
