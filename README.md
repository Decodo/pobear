# pobear · Open-source web scraping CLI with automatic fallback

[![CI](https://github.com/Decodo/pobear/actions/workflows/ci.yml/badge.svg)](https://github.com/Decodo/pobear/actions/workflows/ci.yml)

pobear is an open-source web scraping CLI for extracting structured data and crawling websites. Enable `--ladder` to automatically fall back from direct requests to your configured proxy or scraping API when requests are blocked or rate-limited.

It runs as a single binary, with built-in extraction and reusable scraping jobs. Start without installing a language runtime or creating an account, and keep the same extraction rules as you change fetching methods.

- **Collect the fields you need**. Extract text and attributes with CSS selectors, or convert HTML to Markdown.
- **Build repeatable workflows**. Save commands as JSON job specs, edit them, and run them from scripts or an AI agent.
- **Control your crawl**. Set depth, page limits, scope, and request rates, with sitemap discovery and `robots.txt` support.
- **Use additional fetching services when needed**. Keep direct requests for accessible pages and add a proxy or Decodo for more demanding targets.

## Quick start

Install on Linux or macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/Decodo/pobear/main/install.sh | sh
```

On Windows, download and extract the ZIP archive for your system from the [releases page](https://github.com/Decodo/pobear/releases). Choose `windows_amd64` for x64 or `windows_arm64` for ARM64, then run `pobear.exe`. The shell examples below use POSIX syntax.

Crawl a sample bookshop, extract titles and prices, and save the results:

```sh
pobear crawl https://books.toscrape.com/ --depth 1 --max-pages 10 \
  --css 'title=h3 a@title' --css 'price=.price_color' --out books.ndjson
```

Each fetched page becomes an NDJSON record in `books.ndjson`. The `data` field contains the extracted values. For example, an excerpt from a listing page:

```json
{
  "title": ["A Light in the Attic", "Tipping the Velvet"],
  "price": ["£51.77", "£53.74"]
}
```

Here, `h3 a@title` reads each book link's `title` attribute, while `.price_color` extracts the price text. Change the URL and selectors to collect the fields your workflow needs.

If you have `jq`, inspect the first record's extracted fields:

```sh
head -1 books.ndjson | jq '.data'
```

Without `--out`, results stream to stdout. Run summaries and failure details go to stderr, keeping them separate from the data you pipe or save.

Prefer an interactive interface? Run `pobear` with no arguments to open the terminal wizard, configure a scrape or crawl, inspect results, and save a job.

## What you can build

### Collect web content for research or an LLM

Crawl a documentation site and convert its pages to Markdown:

```sh
pobear crawl https://go.dev/doc/ --depth 1 --max-pages 20 \
  --extract markdown --out docs.ndjson
```

Each record includes the page URL and its Markdown in `data.markdown`. Use the output for search, summarization, or a knowledge base. Crawls stay on the same host by default; use the [crawl options](docs/CLI_REFERENCE.md#pobear-crawl-url) to control scope and filter links.

For a single page without the result envelope:

```sh
pobear scrape https://go.dev/doc/ --extract markdown --bare
```

### Scrape a list of URLs

Create a text file with one URL per line, then use it as the input:

```sh
pobear scrape urls.txt --extract markdown --out pages.ndjson
```

This is useful when another step in your workflow already produces the URLs. Blank lines and lines beginning with `#` are ignored.

### Save a reusable scraping job

Print a job spec instead of executing the command:

```sh
pobear crawl https://books.toscrape.com/ --depth 1 --max-pages 10 \
  --css 'title=h3 a@title' --css 'price=.price_color' \
  --out books.ndjson --print-spec > books.json
```

Run the saved job whenever you need fresh data:

```sh
pobear run books.json
```

`--print-spec` produces JSON; `pobear run` accepts both JSON and YAML. Store jobs in version control, share them with a teammate, or execute them on a schedule. See the [job spec reference](docs/JOB_SPEC_REFERENCE.md) for the document structure and examples.

## Choose how to fetch data

| Method | When to use it | Setup |
| --- | --- | --- |
| Direct request | Pages accessible without an external fetching service | No account required |
| Your own proxy | Requests that need to go through a web scraping proxy | Add `--proxy '<proxy-url>'` |
| Decodo Web Data API | Pages requiring JavaScript rendering, proxy rotation, or bot protection handling | Run `pobear auth decodo`, then add `--provider decodo` |

Use `--proxy` or `--provider` to select a fetching method directly. Add `--ladder` to start with direct fetching and fall back through the alternatives you configure.

For example, route a Markdown extraction through your proxy:

```sh
pobear scrape https://go.dev/doc/ --extract markdown \
  --proxy '<proxy-url>' --out page.ndjson
```

Replace `<proxy-url>` with your proxy's connection URL. Direct and proxy requests support Chrome, Firefox, and Safari TLS fingerprint profiles through `--profile`; the default is Chrome. These profiles do not execute JavaScript.

Decodo is included in the binary. To add another scraping API, create a YAML configuration file using the [provider reference](docs/PROVIDER_REFERENCE.md).

### Extend your workflow with Decodo

Use Decodo's [Web Data API](https://decodo.com/scraping/web?utm_source=github&utm_medium=social&utm_campaign=pobear) to retrieve pages, then apply the same pobear extraction and output options.

1. Copy your Web Data API key from the [Decodo dashboard](https://dashboard.decodo.com/).
2. Run `pobear auth decodo`. At the `decodo token:` prompt, paste your API key and press Enter. pobear saves it for later runs.
3. Add `--provider decodo` to your scrape or crawl command.

For example, authenticate with Decodo, then use its Web Data API to extract book titles and prices and save the results to books.ndjson:

```sh
pobear auth decodo
pobear scrape https://books.toscrape.com/ \
  --css 'title=h3 a@title' --css 'price=.price_color' \
  --provider decodo --out books.ndjson
```

Enter your Decodo authentication token at the prompt. pobear saves it and uses the configured provider settings to authenticate subsequent requests.

The Decodo configuration defaults to the `universal` target, no JavaScript rendering, and the Premium proxy pool (`proxy_pool=premium`). `--render` turns rendering on (`headless=html`). To inspect the available settings:

```sh
pobear scrape --provider decodo --help
```

For scripts and agents, use `pobear providers decodo --json` to read the supported templates, options, and defaults as structured data. Third-party service costs are separate from pobear. See the [Web Data API documentation](https://help.decodo.com/docs/web-data-api-introduction) for service details.

### Fall back automatically when requests are blocked

After authenticating with Decodo, add `--ladder` to try direct fetching first and use Decodo if a request remains blocked or rate-limited after retries:

```sh
pobear crawl https://books.toscrape.com/ --depth 1 --max-pages 10 \
  --css 'title=h3 a@title' --css 'price=.price_color' \
  --ladder --provider decodo --out books.ndjson
```

Add `--proxy '<proxy-url>'` as well to try your proxy between direct fetching and Decodo. Each result's `tier` field identifies the method that retrieved the page.

Fallback responds to blocked and rate-limited fetches. It does not trigger on empty extraction or a challenge page that returns HTTP `200`; see [troubleshooting](#troubleshooting).

## Automate web scraping with AI agents

Run pobear as a subprocess from an agent or script. The binary exposes its commands, output contract, and exit codes as machine-readable documentation:

```sh
pobear docs --json
```

Generate a job spec and pass it directly to the runner:

```sh
pobear scrape https://books.toscrape.com/ --css 'price=.price_color' \
  --print-spec | pobear run -
```

- **Discover provider settings**. Use `pobear providers decodo --json` to read the provider's supported options.
- **Process results**. Read NDJSON from stdout and run summaries and failure details from stderr.
- **Handle failures**. Check the exit code before processing a run's results.
- **Catch missing data**. Add `--fail-on-empty` when an all-empty extraction should fail the job.

| Exit code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | Missing or rejected provider credentials |
| `3` | Invalid command usage or job spec |
| `4` | Partial failure, with some results produced |
| `5` | Fatal failure |

See [driving pobear from an agent](docs/CLI_REFERENCE.md#driving-pobear-from-an-agent) for the full contract and examples.

## Troubleshooting

A failed HTTP request is reported as a failure. A challenge page can instead return HTTP `200` with HTML that contains none of the fields you requested. The fetch succeeds, but the extraction can still be empty:

```text
fetched 1, errors 0, skipped 0, empty 1
```

Check the retrieved content and your selectors before changing the fetching method. Empty extraction can indicate a changed page layout, content that requires rendering, or a challenge page.

For pipelines, add `--fail-on-empty` to return exit code `5` when every fetched item has an empty extraction:

```sh
pobear scrape https://books.toscrape.com/ --css 'price=.price_color' \
  --fail-on-empty --out books.ndjson
```

## FAQ

### Do I need a Decodo account or proxy to use pobear?

No. Direct fetching works without an external account. You need proxy access or provider credentials only when you configure those fetching methods.

### Can pobear scrape JavaScript-rendered websites?

Direct fetching retrieves HTML without executing JavaScript. Use `--provider decodo` for rendered content; the included Decodo configuration requests rendered HTML by default.

### What output formats does pobear support?

The default is one NDJSON object per fetched page, including page metadata, content, and extracted fields. Job specs can also save a JSON array or YAML sequence. Use `--bare` for a single extracted value, or `--out results/` to save each target separately with an `index.json` file.

### Does pobear switch fetching methods automatically?

Only when escalation is enabled. For CLI commands, add `--ladder` with your proxy or provider settings. In the terminal wizard, configuring multiple fetching tiers enables escalation automatically.

### Can I use pobear with an AI agent?

Yes. Agents can invoke the CLI as a subprocess, discover its contract with `pobear docs --json`, and process structured output and exit codes. See [agent automation](#automate-web-scraping-with-ai-agents) for an example.

## Documentation

- [CLI reference](docs/CLI_REFERENCE.md) – Commands, flags, and exit codes.
- [Job spec reference](docs/JOB_SPEC_REFERENCE.md) – Save, edit, and rerun scraping jobs.
- [Provider reference](docs/PROVIDER_REFERENCE.md) – Configure scraping API providers.
- [Project structure](docs/PROJECT_STRUCTURE.md) – Architecture and package layout.
- [Releasing](docs/RELEASING.md) – Release process and versioning.

## Installation options

<details>
<summary>Version pinning, install paths, and PATH handling</summary>

The installer downloads the latest release for your platform, verifies its checksum, checks that the binary runs, and installs it to `$HOME/.local/bin`.

If the directory isn't on your `PATH`, it tries to symlink the binary into a directory that is. Otherwise, it updates your shell's startup file and explains how to activate the change.

| Variable | Default | Effect |
| --- | --- | --- |
| `POBEAR_VERSION` | Latest release | Pin a release tag, such as `v0.1.0` |
| `POBEAR_INSTALL_DIR` | `$HOME/.local/bin` | Choose the install directory |
| `POBEAR_NO_MODIFY_PATH` | Unset | Set to `1` to print PATH instructions without creating symlinks or editing startup files |
| `POBEAR_FORCE` | Unset | Set to `1` to overwrite a different program already named `pobear` |

</details>

## Development

<details>
<summary>Build and check pobear from source</summary>

Requires Go, as specified in `go.mod`, and [golangci-lint](https://golangci-lint.run/) v2. `make ci` also requires ShellCheck. From the repository root:

```sh
make build   # Build ./bin/pobear
make test    # Run Go tests
make vet     # Run go vet
make lint    # Run golangci-lint
make fmt     # Format with gofmt and goimports
make ci      # Run Go and installer checks, tests, and build
```

</details>

pobear is maintained by [Decodo](https://decodo.com/). To report a problem or suggest a feature, [open an issue](https://github.com/Decodo/pobear/issues).

## License

pobear is licensed under the [MIT License](LICENSE).
