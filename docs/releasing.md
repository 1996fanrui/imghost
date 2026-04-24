# Releasing filehub

Maintainer-facing runbook for cutting filehub releases. End-user install
instructions live in [`installation.md`](installation.md).

## Channel semantics

filehub ships on two release channels:

- **alpha** — prerelease. Created by `scripts/release.sh alpha`. The resulting
  GitHub Release is flagged as a prerelease (`gh release create --prerelease`)
  and is not advertised by `install.sh` unless the caller opts in. Alphas are
  the integration testbed for an upcoming minor version.
- **stable** — promoted from the latest alpha of the same base version. Created
  by `scripts/release.sh stable`. The tag drops the `-alpha.N` suffix
  (`v0.1.0-alpha.3` → `v0.1.0`). Stable is what `install.sh` resolves by
  default.

A stable release must always be preceded by at least one alpha on the same
base version; `release.sh stable` refuses to run otherwise.

## `scripts/release.sh` usage matrix

| Scenario | Command | Resulting tag |
|----------|---------|---------------|
| First alpha | `release.sh alpha` | `v0.1.0-alpha.1` |
| Iterate alpha | `release.sh alpha` | `v0.1.0-alpha.2` |
| Promote to stable | `release.sh stable` | `v0.1.0` |
| Start new minor | `release.sh alpha 0.2.0` | `v0.2.0-alpha.1` |

The script:

1. Resolves the next tag by scanning existing `v*` tags.
2. Creates the annotated tag locally.
3. `git push origin <tag>`.
4. `gh release create <tag>` (with `--prerelease` for alpha).

The GitHub Release `published` event then kicks off the build workflow — the
script itself does not produce binaries.

## CI workflow overview

`.github/workflows/release.yml` is triggered by `release: { types: [published] }`.
For each published release it:

1. Resolves the version from the tag name.
2. Runs a `3 OS × 2 arch` build matrix (linux/darwin/windows × amd64/arm64)
   against both `cmd/filehub` and `cmd/filehubd`, producing **12 binaries**.
3. Computes `SHA256SUMS` across all artifacts.
4. Uploads the 12 binaries plus `SHA256SUMS` as release assets via
   `gh release upload`.

`install.sh` / `install.ps1` rely on this asset naming scheme —
`<bin>_<os>_<arch>[.exe]` — so changes to the matrix must be made in lockstep
with the installer scripts.

## Rollback

To retract a tag (e.g. a broken alpha):

```bash
gh release delete <tag> --yes && git push origin ":refs/tags/<tag>"
```

This removes the GitHub Release (including uploaded assets) and deletes the
remote tag. Local tags on contributor clones must be pruned separately
(`git fetch --prune --prune-tags`). Once deleted, the same tag name can be
re-cut by rerunning `release.sh`.
