# pobear

[![CI](https://github.com/Decodo/pobear/actions/workflows/ci.yml/badge.svg)](https://github.com/Decodo/pobear/actions/workflows/ci.yml)

Scrape and crawl the web from your terminal, and get structured data back.

One binary with no runtime to install. Fetch directly, through your own proxy,
or through a third-party scraping API — the same job spec runs on all three, so
you only reach for a paid tier on the sites that need one.

- **Structured output** — NDJSON on stdout, run summary on stderr, so pipes stay clean
- **Extraction built in** — CSS selectors and HTML-to-markdown, no parser to wire up
- **Crawling that behaves** — depth, scope, `robots.txt`, sitemaps, rate limits
- **Pluggable fetching** — direct, proxy, or a scraping API described by a manifest
- **Jobs are documents** — print any command as a spec, store it, re-run it later
- **Built for scripts and agents** — meaningful exit codes and a machine-readable contract

## Why pobear

- **Over `curl` and a parser** — retries, concurrency, rate limiting, `robots.txt`
  and extraction are already there, and the output is structured.
- **Over a hosted scraping API** — most pages do not fight back. Fetch those
  yourself and pay for a provider only where it is actually needed.
- **Over a scraping script** — a job is a document, not code: print it, diff it,
  check it in, hand it to someone else.

Typical uses: pulling product or listing data into a pipeline, turning
documentation into markdown for an LLM corpus, crawling a site politely, or
driving any of it from an agent as a subprocess.

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

## Quick start

Scrape one page and print a single extracted value:

```sh
$ pobear scrape https://example.com --css 'title=h1' --bare
Example Domain
```

Drop `--bare` and each fetched page is one NDJSON object on stdout, with the
run summary on stderr — so redirecting stdout captures just the data:

```sh
$ pobear scrape https://example.com --css 'title=h1' > pages.ndjson
fetched 1, errors 0, skipped 0
```

Turn a page into markdown, or follow its links:

```sh
$ pobear scrape https://example.com --extract markdown --bare
# Example Domain

$ pobear crawl https://example.com --depth 1 --extract markdown --out pages.ndjson
fetched 1, errors 0, skipped 1
```

`--css`, `--extract`, `--depth` and the rest are in
[`docs/CLI_REFERENCE.md`](docs/CLI_REFERENCE.md).

## A worked example

Crawl a bookshop and pull the title and price off every listing:

```sh
$ pobear crawl https://books.toscrape.com/ --depth 1 --max-pages 10 \
    --css 'title=h3 a@title' --css 'price=.price_color' --out books.ndjson
fetched 10, errors 0, skipped 64

$ head -1 books.ndjson | jq '.data'
{
  "title": ["A Light in the Attic", "Tipping the Velvet", …],
  "price": ["£51.77", "£53.74", …]
}
```

Every command is also a document. Print it instead of running it, and you get a
job spec you can store, edit and re-run — and `run` accepts it unchanged:

```sh
$ pobear crawl https://books.toscrape.com/ --depth 1 --print-spec > job.yaml
$ pobear run job.yaml
```

[`examples/job.yaml`](examples/job.yaml) is that same job written out longhand,
with every field spelled out rather than left to its default.

## When a site blocks you

A site that refuses outright is reported as such, and the run exits non-zero:

```sh
$ pobear scrape https://example.com/blocked
https://example.com/blocked [blocked]: tiers: http status 403: automated traffic detected
fetched 0, errors 1, skipped 0          # exit 5
```

A site that serves an interstitial challenge is harder, because it answers with
a normal `200` and real HTML. pobear cannot call that blocked — but it can tell
you the selectors you asked for found nothing:

```sh
$ pobear scrape https://example.com/product --css 'price=.price'
fetched 1, errors 0, skipped 0, empty 1
```

`empty` equal to the fetched count means the run worked mechanically and
produced no data. Add `--fail-on-empty` to make that exit non-zero in a pipeline.

Either way, the fix is the same: move up a tier. `--proxy <url>` routes through
your own proxy; `--provider <name>` goes through a scraping API that handles
anti-bot for you, after one `pobear auth <name>`. The rest of the command is
unchanged, so only the fetch tier moves.

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
