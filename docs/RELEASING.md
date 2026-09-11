# Releasing (spec)

Releases are cut from git tags and published as GitHub Releases by
[goreleaser](https://goreleaser.com), driven by `.goreleaser.yml` and
`.github/workflows/release.yml`.

## Cutting a release

```
$ git tag v0.1.0
$ git push origin v0.1.0
```

Pushing a tag matching `v*` triggers the `release` workflow, which runs
`goreleaser release --clean` and publishes a GitHub Release with:

- `pobear` binaries for `linux`/`darwin`/`windows`, each for `amd64`/`arm64`
  (as `.tar.gz`, or `.zip` for Windows)
- a `checksums.txt` covering every archive
- a changelog generated from the commit messages between this tag and the
  previous one (merge commits and any commit starting `docs:`/`test:` are
  excluded)

Tags are plain [semver](https://semver.org) with a `v` prefix (`v0.1.0`,
`v1.2.3`) — no other tag shape is recognized by the release workflow.

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
