# pobear

A CLI and engine for scraping and crawling the web, with a pluggable
fetch layer (direct, proxy, or third-party scraping API) and a wire
protocol for driving it from any language SDK.

Status: early development — most of what's documented in `/docs` doesn't
exist as code yet. This repo currently holds the design specs and the
project skeleton.

## Documentation

- [`docs/CLI_REFERENCE.md`](docs/CLI_REFERENCE.md) — commands, flags, exit codes
- [`docs/JOB_SPEC_REFERENCE.md`](docs/JOB_SPEC_REFERENCE.md) — the job spec document
- [`docs/PROVIDER_REFERENCE.md`](docs/PROVIDER_REFERENCE.md) — the provider manifest format
- [`docs/PROJECT_STRUCTURE.md`](docs/PROJECT_STRUCTURE.md) — architecture and package layout

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
