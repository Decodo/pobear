# pobear CLI reference (spec)

pobear - short for polar bear. Reasoning, polar bears have one of the best sense of smell in the animal kingdom, which allows them to find ~~data~~ food in the most extreme environments.

---

## `pobear version`

No flags.

```
$ pobear version
pobear 0.0.1
```

---

## `pobear scrape <url|file>`

| Flag | Default | Notes |
|---|---|---|
| `--proxy <url>` | — | tier 1 |
| `--provider <name>` | — | tier 2 |
| `--template <name>` | the provider's `default` template, if any | selects one of the provider's declared templates — see below |
| `--profile chrome\|firefox\|safari` | `chrome` | local/proxy TLS fingerprint |
| `--render` | off | sugar for `--opt render=true` |
| `--geo <code>` | — | sugar for `--opt geo=<code>` |
| `--opt name=value` | — | repeatable; set any option the chosen provider declares — see below |
| `--timeout <duration>` | `30s` | per-request timeout |
| `--extract raw\|markdown` | — | add an extraction stage |
| `--css name=selector[@attr]` | — | repeatable |
| `--out <path>` | stdout | write NDJSON to a file instead |
| `--output-format raw\|json` | `json` | `raw` prints just the extracted value, no envelope — see below |
| `--print-spec` | off | print the spec, don't run it |
| `--concurrency <n>` | 8 | |
| `--retries <n>` | 3 | |
| `--rps <n>` | 0 (unlimited) | requests/sec across every host combined |

```
$ pobear scrape https://ip.decodo.com --print-spec
{
  "version": 1,
  "source": { "urls": ["https://ip.decodo.com"] },
  "fetch": { "use": "local", "profile": "chrome", "timeout": "30s", "noRedirects": false },
  "limits": { "concurrency": 8, "rps": 0, "retries": 3, "backoff": "", "pluginConcurrency": 0 },
  "sink": { "use": "stdout" }
}
```

```
$ pobear scrape https://example.com --extract markdown
{"url":"https://example.com","finalUrl":"https://example.com","status":200,
 "headers":{...},"body":"<!doctype html>…","depth":0,
 "fetch":{"via":"local","attempts":1,"ms":142},
 "data":{"markdown":"# Example Domain\n\nThis domain is for use in…"}}
fetched 1, errors 0, skipped 0          # ← this line is on stderr
```

```
$ pobear scrape https://example.com --css 'title=h1' --css 'link=a@href'
{"url":"https://example.com", …, "data":{"title":"Example Domain","link":"https://www.iana.org/domains/example"}}
```

### `--output-format raw` — just the extracted value, no envelope

Valid only when exactly one URL is targeted and exactly one extraction result would
otherwise land in `data` — one bare `--extract` stage, or exactly one `--css` field.
Anything more (a URL list file, or more than one field) is ambiguous and rejected.

```
$ pobear scrape https://example.com --extract markdown --output-format raw
# Example Domain

This domain is for use in illustrative examples...
```

```
$ pobear scrape https://example.com --css 'title=h1' --css 'link=a@href' --output-format raw
pobear: --output-format raw requires exactly one extracted value, but 2 fields are set (title, link)
$ echo $?
3
```

### `--opt name=value` — provider-declared fields

Every provider manifest declares its own set of accepted fields (types, allowed
values, defaults). `--render`/`--geo` are just the two common ones promoted to
dedicated flags; anything else a provider accepts goes through `--opt`, validated
against that provider's own declared options — not a fixed list built into the CLI.

```
$ pobear scrape https://example.com --provider decodo --opt device=mobile --print-spec
{
  "version": 1,
  "source": { "urls": ["https://example.com"] },
  "fetch": { "use": "provider", "name": "decodo", "options": { "device": "mobile" } },
  "limits": { "concurrency": 8, "rps": 0, "retries": 3, "backoff": "", "pluginConcurrency": 0 },
  "sink": { "use": "stdout" }
}
```

```
$ pobear scrape https://example.com --provider decodo --opt sessionId=abc
pobear: decodo has no option "sessionId"
$ echo $?
3

$ pobear scrape https://example.com --opt render=true          # no --provider set
pobear: spec: fetch.options requires fetch.use: provider, got "local"
$ echo $?
3
```

### `--template <name>` — when a provider has more than one mode

Some providers accept a completely different field set depending on what kind of
page you're asking for — Decodo's `universal` template takes `render`/`geo`; a
hypothetical `amazon_product` template on the same provider would take `domain`
instead and hit a different endpoint entirely. `--opt` is always validated against
whichever template is currently selected, not the provider's full field list.

```
$ pobear scrape https://example.com --provider decodo --template amazon_product --opt domain=co.uk --print-spec
{
  "version": 1,
  "source": { "urls": ["https://example.com"] },
  "fetch": { "use": "provider", "name": "decodo", "template": "amazon_product", "options": { "domain": "co.uk" } },
  "limits": { "concurrency": 8, "rps": 0, "retries": 3, "backoff": "", "pluginConcurrency": 0 },
  "sink": { "use": "stdout" }
}
```

```
$ pobear scrape https://example.com --provider decodo --opt domain=co.uk    # no --template: wrong mode for this field
pobear: decodo template "universal" has no option "domain" (did you mean --template amazon_product?)
$ echo $?
3
```

If a provider declares only one template, or marks one `default: true`, omitting
`--template` just uses it — the flag only becomes required when there's a real
choice to make.

### Discovering a provider's fields — built for agents, not just humans

Free-text `--help` is fine for a person skimming, but fragile for a program parsing
it. Two ways to find out what a provider accepts, one for each:

```
$ pobear scrape --provider decodo --help
...generic scrape flags...

Provider "decodo" — 2 templates (default: universal). Pass --template to switch.

  universal (default)
    --opt render=<bool>                default: false
    --opt geo=<string>                 pattern: ^[a-z]{2}$

  amazon_product
    --opt domain=<enum: com|co.uk|de>  default: com
```

```
$ pobear providers decodo --json
{"name":"decodo","templates":[
  {"name":"universal","default":true,"options":[
    {"name":"render","type":"bool","default":false},
    {"name":"geo","type":"string","pattern":"^[a-z]{2}$"}]},
  {"name":"amazon_product","options":[
    {"name":"domain","type":"enum","values":["com","co.uk","de"],"default":"com"}]}
]}
```

`--json` is the stable contract for programmatic use — it's the manifest's own
validated schema, re-serialized, not text scraped from a help screen. An agent
driving `pobear` should call this once per provider it plans to use, not parse
`--help`.

---

## `pobear crawl <url>`

Everything `scrape` has, plus:

| Flag | Default | Notes |
|---|---|---|
| `--depth <n>` | 1 | link-following depth; `0` = seeds only |
| `--max-pages <n>` | 100 | |
| `--max-duration <duration>` | no limit | stop the whole crawl after this long, even if `max-pages` isn't reached |
| `--scope same-host\|same-domain\|any` | `same-host` | |
| `--include <glob>` | — | repeatable |
| `--exclude <glob>` | — | repeatable; wins over include |
| `--sitemap` | off | also seed from `/sitemap.xml` |
| `--ignore-robots` | off | `robots.txt` is respected by default |
| `--no-redirects` | off | treat a `3xx` response as a dead end instead of following it |
| `--rps <n>` | **2** | (different default from `scrape`) |

`--timeout` (above, inherited from `scrape`) caps one fetch; `--max-duration` caps the
whole crawl — a crawl can have thousands of individually-fast fetches and still run
long past the point you wanted it to stop.

```
$ pobear crawl https://example.com --depth 2 --print-spec
{
  "version": 1,
  "source": { "crawl": { "seeds": ["https://example.com"], "depth": 2,
    "maxPages": 100, "maxDuration": "", "scope": "same-host", "sitemap": false, "robots": "respect" } },
  "fetch": { "use": "local", "profile": "chrome", "timeout": "30s", "noRedirects": false },
  "limits": { "concurrency": 8, "rps": 2, "retries": 3, "backoff": "", "pluginConcurrency": 0 },
  "sink": { "use": "stdout" }
}
```

```
$ pobear crawl https://example.com --depth 1 --extract markdown --out pages.ndjson
fetched 2, errors 0, skipped 0
$ cat pages.ndjson
{"url":"https://example.com/","finalUrl":"https://example.com/","parentUrl":"","status":200,"depth":0,"data":{"markdown":"# Example Domain..."}}
{"url":"https://example.com/about","finalUrl":"https://example.com/about","parentUrl":"https://example.com/","status":200,"depth":1,"data":{"markdown":"# About..."}}
```

`parentUrl` is the page a URL was discovered on — empty for a seed, otherwise the
`url` of whichever fetched page linked to it. Useful for debugging a crawl's shape
and for reconstructing the link graph afterward; it's carried on every item
regardless of extraction stage, not just when crawling.

---

## `pobear run <job.yaml|->`

| Flag | Default | Notes |
|---|---|---|
| `--print-spec` | off | print the parsed spec, don't run it |

Pass `-` instead of a path to read the spec from stdin — which means the output of
`--print-spec` from any other command is itself a valid input to `run`:

```
$ pobear run examples/job.yaml
{"url":"https://books.toscrape.com/","status":200, …}
{"url":"https://books.toscrape.com/catalogue/page-2.html","status":200, …}
fetched 50, errors 0, skipped 0
```

```
$ pobear scrape https://ip.decodo.com --print-spec | pobear run -
{"url":"https://ip.decodo.com","status":200, …}
fetched 1, errors 0, skipped 0
```

```
$ echo 'fetch: {use: local}' > bad.yaml     # neither urls nor crawl set
$ pobear run bad.yaml
pobear: spec: source must set exactly one of urls or crawl, but neither is set
$ echo $?
3
```

---

## `pobear providers [name]`

| Flag | Default | Notes |
|---|---|---|
| `--json` | off | machine-readable schema instead of the human-readable table — see below |

With no name, lists every installed manifest, one line each:

```
$ pobear providers
decodo       https://scraper-api.decodo.com/v2/scrape templates: universal (default), amazon_product
```

```
$ pobear providers                    # no manifests installed
no providers installed (looked in providers)
```

With a name, describes that one manifest in full — every template, every option,
its type, constraints, and default:

```
$ pobear providers decodo
decodo — https://scraper-api.decodo.com/v2/scrape

  universal (default)
    render   bool                              default: false
    geo      string   pattern: ^[a-z]{2}$

  amazon_product
    domain   enum: com, co.uk, de              default: com
```

`--json` prints the same information as a stable, parseable document instead —
see the `--template` section above for an example. This is the recommended way for
an agent to learn a provider's fields; `--help`/the table above are for a human
skimming, not for a program to parse.

```
$ pobear providers bogus-provider
pobear: unknown provider "bogus-provider"
$ echo $?
3
```

---

## `pobear auth <provider>`

No flags. Prompts for a secret on stderr (not echoed).

```
$ pobear auth decodo
decodo token: ****************
saved POBEAR_PROVIDER_DECODO_TOKEN to /Users/you/.pobear/credentials
```

```
$ pobear auth bogus-provider
pobear: unknown provider "bogus-provider"
$ echo $?
3
```

---

## `pobear` (no arguments) / `pobear wizard`

Either launches interactive mode: a full-screen terminal UI, not a one-shot Q&A.
Treat every screen below as an illustration of intended behavior, not a guarantee
of exact wording or layout.

**Navigation model:**
- A main menu of actions. Selecting one opens a **form screen** for it — every
  field for that action is visible and editable at once, in any order, not asked
  one-by-one.
- `esc` always goes back a screen (form → menu), never quits outright.
- Editing one field can change what else is on screen: irrelevant fields disappear,
  newly-relevant fields appear, defaults change to match. The form always reflects
  a single valid job spec for the current field values — there's no state where
  the visible fields don't correspond to something the spec would actually accept.
- Actions (`Test`, `Save`, `Run`) sit at the bottom of every form and act on the
  spec as currently filled in.

### Main menu

```
┌─ pobear ───────────────────────────────────────────┐
│                                                    │
│   › Scrape a page                                  │
│     Crawl a site                                   │
│     Manage providers                               │
│     Manage credentials                             │
│     Quit                                           │
│                                                    │
└────────────────────────────────────────────────────┘
  ↑/↓ move   enter select   q quit
```

### Scrape — form screen, initial state

```
┌─ Scrape ───────────────────────────────────────────────────────── esc: back to menu ─┐
│                                                                                      │
│      URL           https://example.com                                               │
│   › Fetch via     local       (local / proxy / provider)                             │
│      Profile       chrome      (TLS fingerprint)                                     │
│      Extract       markdown    (raw / markdown / fields)                             │
│      Concurrency   8                                                                 │
│      Retries       3                                                                 │
│                                                                                      │
│   [ Test ]   [ Save as job.yaml ]   [ Run ]                                          │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
  ↑/↓ move   enter edit   esc back   ctrl+t test   ctrl+r run
```

### Changing "Fetch via" reacts across the form

Moving "Fetch via" from `local` to `provider`: `Profile` (a local/proxy-only TLS
setting) disappears, and `Provider` plus whatever fields **that provider's manifest
declares** (`DOCUMENTATION.md` §5.1) appear in its place. Decodo declares `render`
and `geo`, so those are what show up below — a provider with a `device` option
instead would show `Device`, not `Render`/`Geo`. The form isn't hardcoded to any
provider's fields; it reads them the same way `--opt` validation does.

```
┌─ Scrape ───────────────────────────────────────────────────────── esc: back to menu ─┐
│                                                                                      │
│      URL           https://example.com                                               │
│      Fetch via     provider    (local / proxy / provider)                            │
│   › Provider      decodo                                                             │
│      Render        off         (decodo option)                                       │
│      Geo           —           (decodo option)                                       │
│      Extract       markdown    (raw / markdown / fields)                             │
│      Concurrency   8                                                                 │
│      Retries       3                                                                 │
│                                                                                      │
│   [ Test ]   [ Save as job.yaml ]   [ Run ]                                          │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

If `Provider` is set to one with no stored credential, pressing `enter` on it opens
the same prompt as `pobear auth decodo` inline, then returns to this form. Changing
`Provider` to a different one re-reads that provider's manifest and swaps the field
list below it accordingly.

### Changing "Extract" reacts the same way

Moving `Extract` to `fields` replaces the single-line field with an editable list:

```
│   Extract     › fields                   (raw / markdown / fields)                   │
│     title         h1                                                                 │
│     price         .price@data-value                                                  │
│     + add field                                                                      │
```

### Test runs in place, on whatever the form currently says

```
→ Testing...
  ✗ Blocked (403) — looks like a bot check, not a real 403.
    "Fetch via" is still "local" — try switching it to "provider" and testing again.

  ✓ 200 OK in 891ms   (after switching Fetch via → provider, Provider → decodo)
    {"title":"Example Domain","price":null}

  ⚠ "price" didn't match anything on the test page — check the selector, or this
    may be normal if the field isn't on every page.
```

There's no separate "blocked path" — a block is just a `Test` result. The form
suggests the fix; the user applies it themselves the same way they'd change any
other field, since every field stays editable at every point.

### Crawl — same form model, different fields and defaults

```
┌─ Crawl ────────────────────────────────────────────────────────── esc: back to menu ─┐
│                                                                                      │
│      Starting URL    https://news.example.com                                        │
│      Fetch via       local       (local / proxy / provider)                          │
│      Profile         chrome                                                          │
│      Depth           2           (0 = seeds only)                                    │
│   › Scope           same-host   (same-host / same-domain / any)                      │
│      Extract         markdown                                                        │
│      Per-host rps    2           (default differs from Scrape)                       │
│      Concurrency     8                                                               │
│      Retries         3                                                               │
│                                                                                      │
│   [ Test ]   [ Save as job.yaml ]   [ Run ]                                          │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

### Field reference

| Form field | Job spec field |
|---|---|
| single page / crawl (menu choice) | `source.urls` vs `source.crawl` |
| URL / Starting URL | `source.urls[]` / `source.crawl.seeds[]` |
| Depth | `source.crawl.depth` |
| Scope | `source.crawl.scope` |
| Fetch via | `fetch.use` |
| Profile | `fetch.profile` (local/proxy only) |
| Provider | `fetch.name` |
| provider-declared fields (e.g. Render / Geo) | `fetch.options.<name>` (provider only, per-manifest) |
| Extract | a stage in the pipeline (`markdown` / `css`) |

---

## `pobear __rpc`

Hidden — no flags, not run by hand. Speaks the wire protocol on stdin/stdout. What SDKs spawn, never what a person types.

---

## Errors that aren't command-specific

```
$ pobear bogus
pobear: unknown command "bogus"
$ echo $?
3

$ pobear scrape --nope
pobear: unknown flag: --nope
$ echo $?
3
```

## Exit codes, at a glance

| Code | Meaning |
|---|---|
| 0 | everything succeeded |
| 2 | auth failure |
| 3 | usage error or invalid spec |
| 4 | partial failure (some items failed) |
| 5 | fatal (everything failed, or unclassified) |
