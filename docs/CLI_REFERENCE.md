# pobear CLI reference (spec)

pobear - short for polar bear. Reasoning, polar bears have one of the best sense of smell in the animal kingdom, which allows them to find ~~data~~ food in the most extreme environments.

---

## `pobear version`

No flags. Prints whatever version string was injected at build time (see
`docs/RELEASING.md`) — `dev` for a plain `go build`/`go run`, the release
tag (e.g. `v0.1.0`) for a binary built by the release pipeline.

```
$ pobear version
pobear dev
```

---

## `pobear scrape <url|file>`

The positional argument is either a single URL, or a path to a file listing many.
Whichever it is is detected by trying to parse the argument itself as an absolute
`http`/`https` URL first; anything else is treated as a file path. A listing file
is plain text, one URL per line: blank lines and lines starting with `#` are
skipped, every other line must be an absolute `http`/`https` URL. Every URL found
(one, or every line of the file) becomes an entry in the resulting `source.urls`.

```
$ cat urls.txt
https://example.com/a
https://example.com/b
# a comment
https://example.com/c

$ pobear scrape urls.txt --print-spec
{
  "version": 1,
  "source": { "urls": ["https://example.com/a", "https://example.com/b", "https://example.com/c"] },
  "fetch": { "use": "local" }
}
```

| Flag | Default | Notes |
|---|---|---|
| `--proxy <url>` | — | tier 1 |
| `--provider <name>` | — | tier 2 |
| `--template <name>` | the provider's `default` template, if any | selects one of the provider's declared templates — see below |
| `--profile chrome\|firefox\|safari` | `chrome` | local/proxy TLS fingerprint; rejected with `--provider` |
| `--render` | off | sugar for `--opt render=true` |
| `--geo <code>` | — | sugar for `--opt geo=<code>` |
| `--opt name=value` | — | repeatable; set any option the chosen provider declares — see below |
| `-t, --timeout <duration>` | `30s` | per-request timeout |
| `-e, --extract raw\|markdown` | — | add an extraction stage |
| `--css name=selector[@attr]` | — | repeatable; each field name must be non-empty and set at most once |
| `-o, --out <path>` | stdout | write NDJSON to a file, or one file per target plus an `index.json` if `<path>` is a directory — see below |
| `-b, --bare` | off | print just the extracted value, no envelope — see below |
| `--print-spec` | off | print the spec, don't run it — fields still at their default are omitted, see below |
| `--print-spec-verbose` | off | like `--print-spec`, but every field is shown fully resolved, defaults included |
| `--fail-on-empty` | off | exit `5` when every fetched item extracted nothing — see below |
| `-c, --concurrency <n>` | 8 | |
| `-r, --retries <n>` | 3 | |
| `--rps <n>` | 0 (unlimited) | requests/sec across every host combined |

Not every flag gets a single-letter shorthand — `pflag` limits a shorthand to exactly
one ASCII character, so it's reserved for flags that are frequent enough to be worth
it and don't collide with something more central. `--proxy`/`--provider`/`--profile`
all start with `p` and are equally central, so none of them claims it rather than
picking a winner; `--render` is itself sugar for `--opt`, so it doesn't need its own
shortcut; `--print-spec`/`--print-spec-verbose`/`--rps` are debugging/advanced knobs
left fully explicit on purpose. `--css` keeps its full name too — its value
(`name=selector[@attr]`) is already the verbose part, so a shorter flag name saves
little. `crawl`'s own flags follow the same reasoning (below) — notably
`--ignore-robots` is deliberately never abbreviated, since it disables a
legal/courtesy default and shouldn't be easy to fat-finger.

### `--print-spec` vs `--print-spec-verbose` — sane defaults, or every field resolved

`--print-spec` omits any field still sitting at its default — the same "every
other field takes its default" spec a hand-written job file is allowed to rely on
(see `JOB_SPEC_REFERENCE.md` §10). `--print-spec-verbose` prints the identical
spec with every default filled in, for when you want to see exactly what the
engine will use without having to hold the defaults table in your head:

```
$ pobear scrape https://ip.decodo.com --print-spec
{
  "version": 1,
  "source": { "urls": ["https://ip.decodo.com"] },
  "fetch": { "use": "local" }
}
```

```
$ pobear scrape https://ip.decodo.com --print-spec-verbose
{
  "version": 1,
  "source": { "urls": ["https://ip.decodo.com"] },
  "fetch": { "use": "local", "profile": "chrome", "timeout": "30s", "noRedirects": false },
  "limits": { "concurrency": 8, "rps": 0, "retries": 3, "backoff": "exponential", "pluginConcurrency": 0, "pluginTimeout": "30s" },
  "stages": [],
  "sink": { "use": "stdout" }
}
```

Both are equally valid input to `pobear run` — the minimal form because every
omitted field has a defined default, the verbose form because it's just that
same document spelled out in full.

```
$ pobear scrape https://example.com --extract markdown
{"url":"https://example.com","finalUrl":"https://example.com","parentUrl":"","status":200,
 "body":"<!doctype html>…","depth":0,
 "data":{"markdown":"# Example Domain\n\nThis domain is for use in…"}}
fetched 1, errors 0, skipped 0          # ← this line is on stderr
```

```
$ pobear scrape https://example.com --css 'title=h1' --css 'link=a@href'
{"url":"https://example.com", …, "data":{"title":["Example Domain"],"link":["https://www.iana.org/domains/example"]}}
```

### `empty` in the summary — extraction came back with nothing

When a job declares stages, the summary counts items whose stages produced no
value at all, and appends `empty N` when that count is non-zero:

```
$ pobear crawl https://example.com --depth 2 --css 'price=.price'
fetched 200, errors 0, skipped 0, empty 200
```

A css field that matched nothing writes an empty list, and `raw`/`markdown` write
an empty string; either counts as empty. An item is only empty when **every**
field came back empty — one field matching is enough to count it as extracted.

This is the closest pobear gets to reporting a soft block. A site that serves an
interstitial challenge page answers with a normal `200` and real HTML, so the
fetch cannot be classified as `[blocked]` the way a `403` can. What pobear can
say is that the selectors you asked for found nothing, which is the same signal a
challenge page, a layout change, or a wrong selector all produce. `empty 200` out
of 200 is unmistakable; a handful is worth a look.

`--fail-on-empty` turns the total case into a non-zero exit, for pipelines that
must not silently record a run of blank items:

```
$ pobear crawl https://example.com --css 'price=.price' --fail-on-empty
fetched 200, errors 0, skipped 0, empty 200
pobear: every fetched item extracted nothing (200 items)
$ echo $?
5
```

It fires only when every fetched item was empty. A run where some items extracted
data is a normal partial result, not a failure.

A per-URL failure prints one detail line per failure before the summary,
naming the URL, its fault kind in brackets, and the underlying error —
same stderr stream, same for `scrape` and `crawl`. For an HTTP-status-based
failure (local/proxy) or a provider-tier failure, that error also includes
a snippet of the actual response body (up to 500 characters), when the
response had one — usually the most direct way to see *why* a request was
rejected, since it's often the server's own explanation:

```
$ pobear scrape https://example.com/blocked
https://example.com/blocked [blocked]: tiers: http status 403: automated traffic detected
fetched 0, errors 1, skipped 0
$ echo $?
5
```

`--extract` and `--css` compose freely — each adds its own stage to the pipeline
(§6 of `JOB_SPEC_REFERENCE.md`), so `--extract markdown --css title=h1` produces
both a stage using `markdown` and a stage using `css`, writing `data.markdown` and `data.title`
in the same item. The only place the combination matters is `--bare` (below), which
requires the *total* result to be a single value across every stage.

### `--bare` — just the extracted value, no envelope

Strips the response down to one bare string, no envelope. Valid only when exactly
one URL is targeted and at most one extraction result would otherwise land in `data`
— one bare `--extract` stage, or exactly one `--css` field. More than one field is
ambiguous and rejected. Since `css` always writes a list (`JOB_SPEC_REFERENCE.md` §6),
`--bare` additionally requires that field's list to hold exactly one element — one
CSS field matching zero or 2+ elements is also rejected as ambiguous, same as multiple
fields.

```
$ pobear scrape https://example.com --extract markdown --bare
# Example Domain

This domain is for use in illustrative examples...
```

If neither `--extract` nor `--css` is set, `--bare` falls back to the raw fetched
body — the same content `--extract raw` would put in `data.raw`, without needing
that stage at all:

```
$ pobear scrape https://example.com --bare
<!doctype html><html>...
```

```
$ pobear scrape https://example.com --css 'title=h1' --css 'link=a@href' --bare
pobear: --bare requires at most one extracted value, but 2 fields are set (title, link)
$ echo $?
3
```

### `--out <path>` — directory mode: one file per target, plus an index

`<path>` is treated as a directory if it already exists as one, or if it ends
in a trailing `/` (or the OS path separator) and hasn't been created yet —
that trailing slash is how you tell the CLI "make this a directory" before it
exists. Anything else — an existing file, or a bare path with no trailing
slash — is file mode: a single NDJSON file, as above.

In directory mode, every target gets its own file, named by a sequential,
zero-padded index in fetch order (`0001`, `0002`, …), not by URL — URLs can
collide or contain characters that are awkward as filenames. Without `--bare`,
each file is that target's full NDJSON line (`0001.json`); with `--bare`, each
file holds just the extracted value (`0001.html`). Unlike file mode, `--bare`
in directory mode does not require exactly one target — and a single item's
extraction failing (its field missing, or a `--css` match producing 0 or 2+
elements) only fails that one entry, not the whole run.

Every target — succeeded, failed, or skipped — also gets an entry in
`index.json`. In `crawl`, a discovered link can be skipped instead of fetched
(out of scope, excluded by `--include`/`--exclude`, disallowed by
`robots.txt`, or the `--max-pages` budget was already spent); a link that's
merely a duplicate of one already seen is silently dropped and never becomes
an entry at all.

```json
[
  { "url": "https://example.com/a", "file": "0001.json", "succeeded": true, "status": 200 },
  { "url": "https://example.com/b", "succeeded": false, "error": "https://example.com/b [blocked]: tiers: http status 403: automated traffic detected" },
  { "url": "https://example.com/c", "succeeded": false, "skipped": true, "error": "https://example.com/c: disallowed by robots.txt" }
]
```

`error` holds the exact same message the stderr failure line would show for
that URL (skipped entries get an analogous one-line reason). Neither a
failed nor a skipped entry has a `file` field, since nothing was written for
it; `skipped` distinguishes "the fetch itself failed" from "this URL was
never attempted by design."

```
$ pobear crawl https://example.com --depth 1 --extract markdown --out ./pages/
fetched 2, errors 1, skipped 0
$ ls ./pages
0001.json  0002.json  index.json
```

### `--opt name=value` — provider-declared fields

Every provider manifest declares its own set of accepted fields (types, allowed
values, defaults). `--render`/`--geo` are just the two common ones promoted to
dedicated flags; anything else a provider accepts goes through `--opt`, validated
against that provider's own declared options — not a fixed list built into the CLI.

```
$ pobear scrape https://example.com --provider decodo --opt geo=us --print-spec
{
  "version": 1,
  "source": { "urls": ["https://example.com"] },
  "fetch": { "use": "provider", "name": "decodo", "options": { "geo": "us" } }
}
```

```
$ pobear scrape https://example.com --provider decodo --opt sessionId=abc
pobear: decodo has no option "sessionId"
$ echo $?
3

$ pobear scrape https://example.com --opt render=true          # no --provider set
pobear: spec: `fetch.options` requires `fetch.use: provider`, got "local"
$ echo $?
3
```

Every `--provider` also needs a saved credential (see `pobear auth <provider>` below) —
without one, the CLI fails before any fetch is attempted:

```
$ pobear scrape https://example.com --provider decodo
pobear: missing credential for provider "decodo" (run: pobear auth decodo)
$ echo $?
1
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
  "fetch": { "use": "provider", "name": "decodo", "template": "amazon_product", "options": { "domain": "co.uk" } }
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

Takes exactly one starting URL — there's no CLI flag to add extra seeds. A crawl
with multiple seeds (`source.crawl.seeds` with 2+ entries) is only reachable by
writing the job spec directly and running it with `pobear run`.

Everything `scrape` has, plus:

| Flag | Default | Notes |
|---|---|---|
| `-d, --depth <n>` | 1 | link-following depth; `0` = seeds only |
| `--max-pages <n>` | 100 | |
| `--max-duration <duration>` | no limit | stop the whole crawl after this long, even if `max-pages` isn't reached |
| `-s, --scope same-host\|same-domain\|any` | `same-host` | |
| `-i, --include <glob>` | — | repeatable |
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
  "source": { "crawl": { "seeds": ["https://example.com"], "depth": 2 } },
  "fetch": { "use": "local" },
  "limits": { "rps": 2 }
}
```

`limits.rps` is printed even though `2` is `crawl`'s own default, because a bare
spec defaults `rps` to `0`. "Omitted" means "`pobear run` would apply this same
value anyway", so any field where `crawl`'s CLI default differs from the spec
default is written out — otherwise piping the spec into `run` would silently
produce a different job.

`--print-spec-verbose` shows the same job with every default filled in, including
the ones `crawl` sets differently than `scrape` (`limits.rps: 2` here, not `0`):

```
$ pobear crawl https://example.com --depth 2 --print-spec-verbose
{
  "version": 1,
  "source": { "crawl": { "seeds": ["https://example.com"], "depth": 2,
    "maxPages": 100, "maxDuration": "", "scope": "same-host", "include": [], "exclude": [],
    "sitemap": false, "robots": "respect" } },
  "fetch": { "use": "local", "profile": "chrome", "timeout": "30s", "noRedirects": false },
  "limits": { "concurrency": 8, "rps": 2, "retries": 3, "backoff": "exponential", "pluginConcurrency": 0, "pluginTimeout": "30s" },
  "stages": [],
  "sink": { "use": "stdout" }
}
```

```
$ pobear crawl https://example.com --depth 1 --extract markdown --out pages.ndjson
fetched 2, errors 0, skipped 0
$ cat pages.ndjson
{"url":"https://example.com/","finalUrl":"https://example.com/","parentUrl":"","status":200,"body":"<!doctype html>…","depth":0,"data":{"markdown":"# Example Domain..."}}
{"url":"https://example.com/about","finalUrl":"https://example.com/about","parentUrl":"https://example.com/","status":200,"body":"<!doctype html>…","depth":1,"data":{"markdown":"# About..."}}
```

`parentUrl` is the page a URL was discovered on — empty for a seed, otherwise the
`url` of whichever fetched page linked to it. Useful for debugging a crawl's shape
and for reconstructing the link graph afterward; it's carried on every item
regardless of extraction stage, not just when crawling.

---

## `pobear run <job.yaml|->`

| Flag | Default | Notes |
|---|---|---|
| `--print-spec` | off | print the parsed spec, don't run it — fields at their default are omitted |
| `--print-spec-verbose` | off | like `--print-spec`, but every field is shown fully resolved, defaults included |

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
pobear: spec: `source` must set exactly one of `urls` or `crawl`, but neither is set
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

## `pobear docs`

| Flag | Default | Notes |
|---|---|---|
| `--json` | off | the same content as a parseable document |

Prints the contract for driving pobear from a script or agent: the command list,
what goes to stdout versus stderr, the fields on an emitted item, the exit codes,
and the named contracts below. It exists because pobear ships as a single binary
— an agent that installed it has no repository, so anything documented only in
`/docs` never reaches it.

```
$ pobear docs --json
{
  "version": "0.1.0",
  "commands": [{"name": "auth", "summary": "Save a credential for a provider"}, ...],
  "exitCodes": [{"code": 0, "meaning": "everything succeeded"}, ...],
  "output": {
    "stdout": "one NDJSON object per fetched item",
    "stderr": "the `fetched N, errors N, skipped N` summary, and one detail line per failed URL",
    "itemFields": ["url", "finalUrl", "parentUrl", "status", "body", "depth", "data"]
  },
  "contracts": [{"id": "provider-schema", "summary": "..."}, ...]
}
```

Each contract carries a stable `id` — `provider-schema`, `spec-round-trip`,
`bare-output`, `empty-extraction`, `soft-block`. The `summary` wording may be
edited; an `id` is the handle a caller keys off, so renaming one is a breaking
change to the contract rather than a rewording.

`exitCodes` is generated from the same constants the CLI exits with, and
`itemFields` is pinned on both sides by tests, so neither can drift from the
behaviour it describes without a test failing.

---

## `pobear auth <provider>`

No flags. Prompts for a secret on stderr. When stdin is a real terminal, the
keystrokes aren't echoed at all as you type (the same as a `sudo` password
prompt — no asterisks either, just nothing shown until you hit Enter). When
stdin isn't a terminal (piped input, e.g. `echo "$TOKEN" | pobear auth decodo`,
or a script/CI feeding it directly), there's nothing to suppress, so it reads
the piped line as-is.

```
$ pobear auth decodo
decodo token:
saved POBEAR_PROVIDER_DECODO_TOKEN to /Users/you/.pobear/credentials
```

```
$ pobear auth bogus-provider
pobear: unknown provider "bogus-provider"
$ echo $?
3
```

An empty secret is rejected and nothing is written, so hitting Enter at the
prompt can't overwrite a working credential with nothing:

```
$ pobear auth decodo
decodo token:
pobear: no secret entered; POBEAR_PROVIDER_DECODO_TOKEN left unchanged
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
declares** (`PROVIDER_REFERENCE.md` §6) appear in its place. Decodo declares `render`
and `geo`, so those are what show up below — a provider with a `device` option
instead would show `Device`, not `Render`/`Geo`. The form isn't hardcoded to any
provider's fields; it reads them the same way `--opt` validation does.

```
┌─ Scrape ───────────────────────────────────────────────────────── esc: back to menu ─┐
│                                                                                      │
│      URL           https://example.com                                               │
│      Fetch via     provider    (local / proxy / provider)                            │
│   › Provider      decodo       (type to search)                                      │
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

(This example is a provider with flat `options:`, no `templates:` — see the next
section for how a templated provider like the real Decodo manifest looks instead.)

Both `Provider` and `Template` (below) support typing to jump straight to a match,
not just cycling one at a time with the arrow keys — useful once either list grows
past a handful of entries. If `Provider` has no stored credential, pressing `enter`
on `[ Run ]` shows an error naming the provider and pointing at `Manage providers`
(or `pobear auth <name>`) instead of starting the run — `Save` never checks, since
writing a spec to disk doesn't call the provider at all. Changing `Provider` to a
different one re-reads that provider's manifest and swaps the field list below it
accordingly, resetting `Template` and its options.

### A provider with more than one template gets a `Template` field too

Any provider that declares two or more templates gets a `Template` field between
`Provider` and the option fields — regardless of whether one is marked `default` —
so every template the CLI's `--template` flag can reach is reachable from the wizard
too, not just the default. It defaults to the manifest's `default: true` template if
one exists, otherwise the first declared template; the real Decodo manifest declares
four (`universal`, marked default, plus `google`/`bing`/`amazon`):

```
┌─ Scrape ───────────────────────────────────────────────────────── esc: back to menu ─┐
│                                                                                      │
│      URL           https://example.com                                               │
│      Fetch via     provider    (local / proxy / provider)                            │
│      Provider      decodo      (type to search)                                      │
│   › Template      universal    (type to search)                                      │
│      Headless      html        (universal option)                                    │
│      Proxy pool    premium     (universal option)                                    │
│      Geo           —           (universal option)                                    │
│      Extract       markdown    (raw / markdown / fields)                             │
│      Concurrency   8                                                                 │
│      Retries       3                                                                 │
│                                                                                      │
│   [ Test ]   [ Save as job.yaml ]   [ Run ]                                          │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

Changing `Template` re-reads that template's declared options and swaps the field
list below it, the same way changing `Provider` does. If the manifest has only one
template, the `Template` field never appears at all — there's nothing to choose.

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

### Manage providers

Lists every loaded manifest — embedded and on-disk merged — with whether a
credential is stored for each (the `✓`/`✗` before the name). `enter` drills
into a manifest's full schema (same information as `pobear providers <name>`,
scrollable); `a` opens a masked input to set or replace that provider's
credential, saving and overwriting immediately on submit, the same as
`pobear auth <name>` does today — there's no separate confirm step; `d`
deletes an on-disk manifest after a `y`/`n` confirmation. A provider that's
only available via the embedded copy (nothing on disk with that name) can't
be deleted from here — there's no file to remove.

```
┌─ Manage providers ─────────────────────────────────────────────────────────────────┐
│                                                                                    │
│  › ✓ decodo    https://scraper-api.decodo.com/v2/scrape                            │
│    ✗ acme      https://acme.example.com/scrape                                     │
│                                                                                    │
└────────────────────────────────────────────────────────────────────────────────────┘
  ↑/↓ move   enter view   a auth   d delete   esc back
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
| Template (multi-template providers only) | `fetch.template` |
| provider-declared fields (e.g. Render / Geo) | `fetch.options.<name>` (provider only, per-manifest, per-template) |
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

A command called without an argument it requires prints that error, then the
command's full help (flags and all) — same as `--help` would, minus the
provider-schema section below:

```
$ pobear scrape
pobear: scrape requires a URL or file argument

Usage:
  pobear scrape <url|file> [flags]

Flags:
  ...
$ echo $?
3
```

## Driving pobear from an agent

Everything an agent needs to run pobear as a subprocess and consume the result
without parsing prose. Each rule below is a contract, not a convention.

The installed binary carries this same contract: `pobear docs` prints it, and
`pobear docs --json` emits it as a parseable document. An agent with only the
binary and no access to this repository should start there.

**Discover a provider's fields from the schema, never from `--help`.**
`--help` is free text for a human skimming; its wording and layout can change.
`providers <name> --json` re-serializes the manifest's own validated schema, so
it is the stable surface:

```
$ pobear providers decodo --json
{"name":"decodo","templates":[{"name":"universal","default":true,"options":[...]}]}
```

**Build a spec, inspect it, then run it.** `--print-spec` prints the job spec
instead of running it, and that output is valid, equivalent input to `run -`.
This lets an agent construct a job from flags, check or store it, and execute it
as a separate step:

```
$ pobear scrape https://example.com --print-spec | pobear run -
```

`--print-spec-verbose` prints the same job with every default filled in. Neither
form requires a provider credential, so a spec can be generated on a machine
that has never run `pobear auth`.

**Results are on stdout, progress is on stderr.** Every fetched item is one
NDJSON object on stdout. The `fetched N, errors N, skipped N` summary and the
per-failure detail lines go to stderr. Redirecting stdout therefore captures
exactly the data and nothing else:

```
$ pobear scrape https://example.com --css 'title=h1' > items.ndjson
fetched 1, errors 0, skipped 0          # ← stderr, still on the terminal
```

**Use `--bare` for a single value.** It drops the envelope and prints just the
extracted value, which suits substitution into another command. It requires at
most one extracted field and rejects anything ambiguous:

```
$ pobear scrape https://example.com --css 'title=h1' --bare
Example Domain

$ pobear scrape https://example.com --css 'a=h1' --css 'b=p' --bare
pobear: --bare requires at most one extracted value, but 2 fields are set (a, b)
```

**Branch on the exit code, not on the message text.** The codes are listed in
full below: `0` success, `1` auth, `3` usage or invalid spec, `4` partial, `5`
fatal. `4` is the one worth handling deliberately — some items succeeded and
their data is on stdout, so a run that exits `4` is still worth consuming.

**Check `empty` before trusting a successful run.** A site that serves an
interstitial challenge page answers with a normal `200` and real HTML, so the
fetch cannot be classified as `[blocked]` the way a `403` can. What pobear can
report is that the stages you asked for extracted nothing:

```
$ pobear scrape https://example.com --css 'price=.price'
fetched 1, errors 0, skipped 0, empty 1
```

`empty N` equal to the fetched count is the signal that a run succeeded
mechanically and produced no data — a challenge page, a layout change, or a
wrong selector all look like this. `--fail-on-empty` turns the all-empty case
into exit `5` for pipelines that must not record a run of blank items.

---

## Exit codes, at a glance

| Code | Meaning |
|---|---|
| 0 | everything succeeded |
| 1 | auth failure |
| 3 | usage error or invalid spec |
| 4 | partial failure (some items failed) |
| 5 | fatal (everything failed, or unclassified) |

`2` is deliberately left unused: it's the exit status Go's own runtime uses for
an unrecovered panic, so a bare `2` from `pobear` always means an actual crash,
never a classified job outcome.
