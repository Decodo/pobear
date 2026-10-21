# Releasing (spec)

Releases are fully automated. Merging a PR to `main` decides the next version
from its title, tags the merge commit, and publishes a GitHub Release with
binaries and release notes. Nobody picks a version number.

Two pieces split the work:

- [semantic-release](https://semantic-release.gitbook.io), configured by
  `.releaserc.json` and run by `.github/workflows/semantic-release.yml`, decides
  the version, writes the release notes and pushes the `vX.Y.Z` tag.
- [goreleaser](https://goreleaser.com), configured by `.goreleaser.yml` and run
  by `.github/workflows/release.yml`, builds the binaries and creates the GitHub
  Release with them attached.

## How a release happens

Every push to `main` runs `semantic-release.yml`:

1. `go build`, `go vet`, `golangci-lint` and `go test` — the same checks PR CI
   (`ci.yml`) runs. A failure here stops the release before anything is tagged.
2. semantic-release reads the commits since the last `vX.Y.Z` tag and picks the
   bump (below). If nothing releasable landed, it stops: no tag, no release.
3. Otherwise it pushes the new tag and hands the tag and its release notes to
   `release.yml`, called directly as a reusable workflow (`on: workflow_call`).
   It has to be called directly rather than triggered by the tag push, because
   the push is made with the default `GITHUB_TOKEN`, and GitHub never lets a
   push made with that token trigger other workflow runs.
4. `release.yml` runs `goreleaser release --clean --release-notes=<notes>`,
   which creates the GitHub Release with the binaries, checksums and
   semantic-release's notes. semantic-release itself never creates the GitHub
   Release, so a Release never exists without its binaries.

`install.sh` fetches `/releases/latest`, so a plain install picks up the new
version as soon as goreleaser finishes.

## Version bumps

Only the commit that lands on `main` counts. PRs are squash-merged, so that is
the PR title, which the `PR title` check (`.github/workflows/pr-title.yml`)
requires to follow [Conventional Commits](https://www.conventionalcommits.org):

| PR title | Release |
|---|---|
| `fix: …` | patch (`0.2.1` → `0.2.2`) |
| `feat: …` | minor (`0.2.1` → `0.3.0`) |
| `feat!: …`, `fix!: …`, or a `BREAKING CHANGE:` footer | major (`0.2.1` → `1.0.0`) |
| `chore:`, `docs:`, `ci:`, `refactor:`, `test:`, `build:`, `perf:`, `style:` | none |

A breaking change releases a new major even while the version is `0.x`.
Going past `v1` also needs the `/vN` suffix on the module path in `go.mod`,
which is not automated.

Nothing is committed back to `main`: no version file, no `CHANGELOG.md`.
Release notes live on the GitHub Releases page.

## Tags

Tags are plain [semver](https://semver.org) with a `v` prefix (`v0.2.1`,
`v1.2.3`). semantic-release computes the next version from the latest such tag
reachable from `main`, and ignores tags with a prerelease suffix (the old
`vX.Y.Z-canary.N` tags).

A human pushing a tag by hand still triggers `release.yml` (a push made by a
person, not `GITHUB_TOKEN`, triggers workflows normally), and goreleaser then
writes its own changelog from `.goreleaser.yml`. semantic-release picks that
tag up as the last release next time. Use this only to recover from a failed
run, not as the normal path.

`.goreleaser.yml`'s `release.prerelease: auto` marks any tag with a prerelease
suffix as a GitHub pre-release; automated releases never have one.

## Version string

`pobear version` prints whatever was injected into `main.version` at build
time via `-ldflags "-X main.version=..."`; goreleaser sets this to the tag
name for a release build. A plain `go build`/`go run` (no ldflags) leaves it
at the `dev` default — this is expected, not a bug, and how you tell a local
build apart from a released one.

## Testing the release pipeline locally

`goreleaser` must be installed locally (`brew install goreleaser`, or see
the [install docs](https://goreleaser.com/install/)). From the repo root:

```
$ goreleaser check           # validates .goreleaser.yml
$ goreleaser release --snapshot --clean   # full local build, no tag/publish required
```

`--snapshot` fakes a version (`0.0.0-SNAPSHOT-<commit>`) so this works
without a real tag, and `--clean` wipes `dist/` first. Nothing is published
in this mode — inspect the built archives under `dist/`.
