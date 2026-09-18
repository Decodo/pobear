# Releasing (spec)

Releases are cut from git tags and published as GitHub Releases by
[goreleaser](https://goreleaser.com), driven by `.goreleaser.yml` and
`.github/workflows/release.yml`. `release.yml` runs the same job
(`goreleaser release --clean`) two ways: triggered by a `v*` tag push (what
happens when a human pushes a tag by hand), or invoked directly as a
reusable workflow (`on: workflow_call`) by `canary.yml`/`promote-stable.yml`
after they push their tag. It has to be invoked directly rather than relying
on the tag push to trigger it, because those two workflows push using the
default `GITHUB_TOKEN` — and GitHub never lets a push made with that token
trigger other workflow runs, to prevent recursive triggering. Either way,
nothing else about the release build differs between a canary and a stable
release.

## Canary releases (automatic)

Every push to `main` (i.e. every merged MR) runs `.github/workflows/canary.yml`,
which:

1. Finds the latest *stable* tag (bare `vX.Y.Z`, no suffix — falls back to
   `v0.0.0` if none exist yet).
2. Bumps its patch version by one and appends `-canary.<run number>`, e.g.
   `v0.1.1-canary.7`.
3. Tags the merge commit, pushes that tag, then calls the `release`
   workflow directly (see above for why a plain push isn't enough here).

A canary gets the same binaries/checksums/changelog as any other release —
it's just marked as a GitHub *pre-release* (see below), and its version
number never collides with an eventual stable release of the same patch,
since a prerelease tag always sorts below its bare-version counterpart in
semver. `install.sh` fetches `/releases/latest`, which GitHub defines to
skip prereleases and drafts, so a canary is never what a plain install
picks up.

## Stable releases (manual)

A human decides when to promote a stable release, by running
`.github/workflows/promote-stable.yml` from the Actions tab
(`workflow_dispatch`) against the desired ref, with a `version` input —
bare semver, no `v` prefix or suffix (e.g. `0.1.1`). It validates that
shape, tags that ref as `v0.1.1`, pushes it, and — the same as canary —
calls the `release` workflow directly rather than relying on the push.

Cutting a release by hand still works via a plain tag push (no direct
`release.yml` call needed, since a push made by an actual person, not
`GITHUB_TOKEN`, does trigger it normally), if you'd rather not go through
the Actions UI:

```
$ git tag v0.1.0
$ git push origin v0.1.0
```

Tags are plain [semver](https://semver.org) with a `v` prefix (`v0.1.0`,
`v1.2.3`, or a canary's `v0.1.1-canary.7`) — no other tag shape is
recognized by the release workflow.

## Canary vs. stable

`.goreleaser.yml`'s `release.prerelease: auto` is what tells them apart —
goreleaser marks a GitHub Release as a pre-release whenever the tag has a
semver prerelease component (anything after a `-`), and as the normal
"Latest" release otherwise. Nothing else about the two workflows is
goreleaser-visible; the distinction is entirely in the tag's shape.

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
