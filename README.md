# pobear

A CLI and engine for scraping and crawling the web, with a pluggable
fetch layer (direct, proxy, or third-party scraping API) and a wire
protocol for driving it from any language SDK.

Status: early development — most of what's documented in `/docs` doesn't
exist as code yet. This repo currently holds the design specs and the
project skeleton.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/Decodo/pobear/main/install.sh | sh
```

Downloads the latest release for your platform, verifies it against the
release `checksums.txt`, runs the installed binary to confirm it works, and
installs it to `$HOME/.local/bin`.

If the install directory isn't already on your `PATH`, the script symlinks the
binary into a directory that is; failing that, it appends the directory to your
shell's startup file and tells you how to activate it in the current shell.

| Variable | Default | Effect |
|---|---|---|
| `POBEAR_VERSION` | latest release | pin a release tag, e.g. `v0.1.0` |
| `POBEAR_INSTALL_DIR` | `$HOME/.local/bin` | where to install |
| `POBEAR_NO_MODIFY_PATH` | unset | `1` never symlinks or edits startup files; prints the `export` line instead |
| `POBEAR_FORCE` | unset | `1` overwrites a different program already named `pobear` |

```sh
POBEAR_VERSION=v0.1.0 POBEAR_INSTALL_DIR=/usr/local/bin sh install.sh
```

Linux and macOS only. On Windows, download a `.zip` from the
[releases page](https://github.com/Decodo/pobear/releases).

## Documentation

- [`docs/CLI_REFERENCE.md`](docs/CLI_REFERENCE.md) — commands, flags, exit codes
- [`docs/JOB_SPEC_REFERENCE.md`](docs/JOB_SPEC_REFERENCE.md) — the job spec document
- [`docs/PROVIDER_REFERENCE.md`](docs/PROVIDER_REFERENCE.md) — the provider manifest format
- [`docs/PROJECT_STRUCTURE.md`](docs/PROJECT_STRUCTURE.md) — architecture and package layout
- [`docs/RELEASING.md`](docs/RELEASING.md) — how a release is cut and where the version string comes from

## For AI agents

pobear is built to be driven as a subprocess. The installed binary carries its
own contract — run `pobear docs`, or `pobear docs --json` for a parseable
document. In short:

- **Discover provider fields** with `pobear providers <name> --json` — the
  manifest's own validated schema. Never parse `--help`; it is free text for
  humans.
- **Build then run**: `pobear scrape URL --print-spec | pobear run -`. The
  printed spec is valid, equivalent input, and printing it needs no credential.
- **Read stdout, watch stderr**: results are NDJSON on stdout; the
  `fetched N, errors N, skipped N` summary and per-failure lines are on stderr.
- **Use `--bare`** to get a single extracted value with no envelope.
- **Branch on exit codes**: `0` success, `1` auth, `3` usage/spec, `4` partial
  (some data was still produced), `5` fatal.
- **Check `empty N`** in the summary before trusting a run — a challenge page
  returns `200` with real HTML, so "fetched 1, empty 1" is how a soft block
  looks. `--fail-on-empty` makes the all-empty case exit non-zero.

Full detail, with worked examples, is in
[`docs/CLI_REFERENCE.md`](docs/CLI_REFERENCE.md#driving-pobear-from-an-agent).

## Development

Requires Go (see `go.mod`) and [golangci-lint](https://golangci-lint.run/) v2.

```sh
make build   # build ./bin/pobear
make test    # go test ./...
make vet     # go vet ./...
make lint    # golangci-lint run
make fmt     # golangci-lint fmt (gofmt + goimports)
make ci      # vet + lint + test + build
```
