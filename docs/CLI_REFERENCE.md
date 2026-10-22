# pobear CLI reference (spec)

pobear - short for polar bear. Reasoning, polar bears have one of the best sense of smell in the animal kingdom, which allows them to find ~~data~~ food in the most extreme environments.

---

## `pobear version`

No flags. Prints whatever version string was injected at build time (see
`docs/RELEASING.md`) — `dev` for a plain `go build`/`go run`, the release
tag (e.g. `v0.1.0`) for a binary built by the release pipeline. `pobear --version`
and `pobear -v` are aliases.

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
| `--ladder` | off | escalate from local through whichever of proxy/provider are configured, if blocked/rate-limited — a no-op with neither set |
| `--template <name>` | the provider's `default` template, if any | selects one of the provider's declared templates — see below |
| `--profile chrome\|firefox\|safari` | `chrome` | local/proxy TLS fingerprint; rejected with `--provider` |
| `--render` | off | sugar for `--opt render=true` |
| `--geo <code>` | — | sugar for `--opt geo=<code>` |
| `--opt name=value` | — | repeatable; set any option the chosen provider declares — see below |
| `-t, --timeout <duration>` | `30s` | per-request timeout |
| `-e, --extract raw\|markdown` | — | add an extraction stage |
| `--css name=selector[@attr]` | — | repeatable; each field name must be non-empty and set at most once |
| `-H, --header "name: value"` | — | repeatable; sets `fetch.headers` (`JOB_SPEC_REFERENCE.md` §4) — each name must be non-empty and set at most once |
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
little. `--header` is the one repeatable, verbose-valued flag that does claim a
letter (`-H`): it's the one place pobear intentionally matches a convention from
outside its own flag set (curl's `-H`) rather than picking freely, since anyone
reaching for a header flag already expects that letter.

`crawl`'s own flags follow the same reasoning (below) — notably
`--ignore-robots` is deliberately never abbreviated, since it disables a
legal/courtesy default and shouldn't be easy to fat-finger.

### `--ladder` — escalating to a higher tier when blocked

`--proxy`/`--provider` pick which additional tiers are configured; `--ladder` decides
whether a `[blocked]` or rate-limited fetch climbs from `local` through whichever of
them are set instead of failing outright (see `JOB_SPEC_REFERENCE.md` §4 for the full
escalation rule). It always starts at `local` regardless of which of `--proxy`/
`--provider` are set, so it's also what lets `--proxy` and `--provider` be set
together at all — without it, that combination is ambiguous (which tier actually
fetches?) and rejected:

```
$ pobear scrape https://example.com --proxy http://p.test --provider decodo
pobear: both --proxy and --provider are set -- add --ladder to escalate between them, or remove one
$ echo $?
3
```

`--ladder` with neither `--proxy` nor `--provider` set is a valid no-op — there's
nothing above `local` to climb to, so it behaves exactly like not passing `--ladder`
at all:

```
$ pobear scrape https://example.com --ladder --print-spec
{
  "version": 1,
  "source": { "urls": ["https://example.com"] },
  "fetch": { "use": "local", "ladder": true }
}
```

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
  "fetch": { "use": "local", "ladder": false, "profile": "chrome", "timeout": "30s", "noRedirects": false },
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
 "data":{"markdown":"# Example Domain\n\nThis domain is for use in…"},"tier":"local"}
fetched 1, errors 0, skipped 0          # ← this line is on stderr
```

```
$ pobear scrape https://example.com --css 'title=h1' --css 'link=a@href'
{"url":"https://example.com", …, "data":{"title":["Example Domain"],"link":["https://www.iana.org/domains/example"]}}
```

### `-H, --header` — extra request headers

Repeatable, `"name: value"` shape (curl's own `-H` convention), each name set at
most once. This is the CLI surface for `fetch.headers` (`JOB_SPEC_REFERENCE.md`
§4) — the only way to set it from `scrape`/`crawl` directly, without hand-writing
a job spec and running it with `pobear run`:

```
$ pobear scrape https://example.com --header 'X-Custom: 1' --print-spec
{
  "version": 1,
  "source": { "urls": ["https://example.com"] },
  "fetch": { "use": "local", "headers": { "X-Custom": "1" } }
}
```

A repeated header name is rejected rather than silently keeping the last one,
same as a repeated `--css` field name:

```
$ pobear scrape https://example.com --header 'X-Custom: 1' --header 'X-Custom: 2'
pobear: --header "X-Custom" is set more than once
$ echo $?
3
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

This is the closest pobear gets to reporting a soft block it doesn't recognize. A
`200` challenge page from a known anti-bot vendor (Amazon's CAPTCHA, Cloudflare,
DataDome, PerimeterX, Google Search) is classified as `[blocked]` like a `403`, so `--ladder`
climbs past it; any other interstitial answers with a normal `200` and real HTML.
What pobear can say is that the selectors you asked for found nothing, which is the same signal a
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

The "exactly one URL" half of the rule is checked just as early, before any
fetch: a `urls.txt` naming more than one URL is rejected the same way, for
the same reason (`scrape <url|file>` is the only way to hand `--bare` more
than one URL — the positional argument takes exactly one URL or one file of
them):

```
$ pobear scrape urls.txt --bare
pobear: --bare requires exactly one URL to be targeted, got 2
$ echo $?
3
```

`crawl` only ever takes one seed argument, but following links can still
discover more than one page, so `--bare` there requires `--depth 0` (seeds
only) — anything deeper is rejected before the crawl starts, not after it
runs:

```
$ pobear crawl https://example.com --bare
pobear: --bare requires --depth 0 -- crawling deeper can discover more than one page, and --bare only ever prints a single value (use --out <dir> to write one file per page instead)
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
| `--sitemap` | **on** | also seed from `/sitemap.xml`; pass `--sitemap=false` to turn off |
| `--ignore-robots` | off | `robots.txt` is respected by default |
| `--no-redirects` | off | treat a `3xx` response as a dead end instead of following it |
| `--rps <n>` | **16** | (different default from `scrape`) |

`--timeout` (above, inherited from `scrape`) caps one fetch; `--max-duration` caps the
whole crawl — a crawl can have thousands of individually-fast fetches and still run
long past the point you wanted it to stop.

```
$ pobear crawl https://example.com --depth 2 --print-spec
{
  "version": 1,
  "source": { "crawl": { "seeds": ["https://example.com"], "depth": 2, "sitemap": true } },
  "fetch": { "use": "local" },
  "limits": { "rps": 16 }
}
```

`limits.rps` and `source.crawl.sitemap` are printed even though `16` and `true` are
`crawl`'s own defaults, because a bare spec defaults `rps` to `0` and `sitemap` to
`false`. "Omitted" means "`pobear run` would apply this same value anyway", so any
field where `crawl`'s CLI default differs from the spec default is written out —
otherwise piping the spec into `run` would silently produce a different job.

`--print-spec-verbose` shows the same job with every default filled in, including
the ones `crawl` sets differently than `scrape` (`limits.rps: 16` here, not `0`):

```
$ pobear crawl https://example.com --depth 2 --print-spec-verbose
{
  "version": 1,
  "source": { "crawl": { "seeds": ["https://example.com"], "depth": 2,
    "maxPages": 100, "maxDuration": "", "scope": "same-host", "include": [], "exclude": [],
    "sitemap": true, "robots": "respect" } },
  "fetch": { "use": "local", "ladder": false, "profile": "chrome", "timeout": "30s", "noRedirects": false },
  "limits": { "concurrency": 8, "rps": 16, "retries": 3, "backoff": "exponential", "pluginConcurrency": 0, "pluginTimeout": "30s" },
  "stages": [],
  "sink": { "use": "stdout" }
}
```

```
$ pobear crawl https://example.com --depth 1 --extract markdown --out pages.ndjson
fetched 2, errors 0, skipped 0
$ cat pages.ndjson
{"url":"https://example.com/","finalUrl":"https://example.com/","parentUrl":"","status":200,"body":"<!doctype html>…","depth":0,"data":{"markdown":"# Example Domain..."},"tier":"local"}
{"url":"https://example.com/about","finalUrl":"https://example.com/about","parentUrl":"https://example.com/","status":200,"body":"<!doctype html>…","depth":1,"data":{"markdown":"# About..."},"tier":"local"}
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
decodo       https://data.decodo.com/v1/scrape templates: universal (default), amazon_product
```

```
$ pobear providers                    # no manifests installed
no providers installed (looked in providers)
```

With a name, describes that one manifest in full — every template, every option,
its type, constraints, and default:

```
$ pobear providers decodo
decodo — https://data.decodo.com/v1/scrape

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
    "itemFields": ["url", "finalUrl", "parentUrl", "status", "body", "depth", "data", "tier"]
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
- Actions (`Save`, `Run`) sit at the bottom of every form and act on the
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

The `URL` field is a small text area, not a single line — paste or type one URL
per line to scrape several pages in one run. `Local`, `Proxy`, and `Provider` are
three rows, one per fetch tier, each starting unconfigured (`— not set`); `Extract`
defaults to `bare` (no extraction stage; the scraped page is returned as-is, same
as CLI's default `--extract ""`).

```
┌─ Scrape ───────────────────────────────────────────────────────── esc: back to menu ─┐
│                                                                                      │
│      URL           (one per line)                                                    │
│                                                                                      │
│   › Local          — not set     (enter to configure)                                │
│      Proxy          — not set     (enter to configure)                               │
│      Provider        — not set     (enter to configure)                              │
│      Extract       bare        (bare / raw / markdown / css fields)                  │
│      Concurrency   8                                                                 │
│      Retries       3                                                                 │
│                                                                                      │
│   [ Save as job.yaml ]   [ Run ]                                                     │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
  ↑/↓ move   enter edit   esc back
```

### Each tier row opens its own screen

Pressing `enter` on `Local`, `Proxy`, or `Provider` pushes a dedicated screen for
that tier, instead of expanding fields inline the way the old single `Fetch via`
choice did — each tier's own fields live there and nowhere else: `Profile` for
`Local`; `Proxy URL` and `Profile` for `Proxy`; `Provider`, `Template`, and that
provider's declared options for `Provider`. `esc` on a tier screen saves whatever
was entered and returns to the main form — opening `Local` and immediately
pressing `esc` is enough to configure it, since it takes no required field of its
own:

```
┌─ Local ────────────────────────────────────────────────────────────────── esc: back ─┐
│                                                                                      │
│   › Profile       chrome      (TLS fingerprint)                                      │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

Back on the main form, the row shows a `✓` and a short summary of that tier's own
configuration once it's been visited — `Local` now reads `✓ chrome`, and stays
`— not set` for any tier never opened:

```
│   › Local          ✓ chrome        (enter to configure)                              │
│      Proxy          — not set     (enter to configure)                               │
│      Provider        — not set     (enter to configure)                              │
```

### Provider — same fields as before, now on their own screen

Selecting `Provider` and pressing `enter` opens a screen with `Provider` plus
whatever fields **that provider's manifest declares** (`PROVIDER_REFERENCE.md`
§6) below it. Decodo's `universal` template declares `headless`, `proxy_pool`,
`geo`, `locale`, and `device_type`, so those are what show up — a provider with a
`device` option instead would show `Device`, not those. The form isn't hardcoded
to any provider's fields; it reads them the same way `--opt` validation does. The
real Decodo manifest declares four templates, so `Provider` here is immediately
followed by a `Template` field:

```
┌─ Provider ─────────────────────────────────────────────────────────────── esc: back ─┐
│                                                                                      │
│      Provider      decodo       (type to search)                                    │
│   › Template      universal    (type to search)                                      │
│      Headless      html        (universal option)                                    │
│      Proxy pool    premium     (universal option)                                    │
│      Geo           —           (universal option)                                    │
│      Locale         —           (universal option)                                   │
│      Device type    —           (universal option)                                   │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

Both `Provider` and `Template` support typing to jump straight to a match, not
just cycling one at a time with the arrow keys — useful once either list grows
past a handful of entries. If `Provider` has no stored credential, pressing
`enter` on `[ Run ]` back on the main form shows an error naming the provider and
pointing at `Manage providers` (or `pobear auth <name>`) instead of starting the
run — `Save` never checks, since writing a spec to disk doesn't call the provider
at all. Changing `Provider` to a different one re-reads that provider's manifest
and swaps the field list below it accordingly, resetting `Template` and its
options. Back on the main form, `Provider`'s summary combines both: `✓ decodo /
universal`.

Any provider that declares two or more templates gets this `Template` field —
regardless of whether one is marked `default` — so every template the CLI's
`--template` flag can reach is reachable from the wizard too, not just the
default. It defaults to the manifest's `default: true` template if one exists,
otherwise the first declared template. Changing `Template` re-reads that
template's declared options and swaps the field list below it, the same way
changing `Provider` does. If the manifest has only one template, the `Template`
field never appears at all — there's nothing to choose.

### Configuring more than one tier turns on escalation automatically

There's no separate `ladder` field to flip. Configuring more than one tier —
`Local` and `Provider`, say — is itself what turns escalation on: `fetch.use`
becomes the lowest configured tier, `fetch.ladder` becomes `true`, and a fetch
that comes back blocked or rate-limited climbs to the next configured tier up
(`JOB_SPEC_REFERENCE.md` §4). Configuring exactly one tier behaves exactly as
before — `fetch.use` is that tier, `ladder` stays `false`:

```
│   › Local          ✓ chrome        (enter to configure)                              │
│      Proxy          — not set     (enter to configure)                               │
│      Provider        ✓ decodo / universal     (enter to configure)                   │
```

This produces `fetch: {use: local, ladder: true, name: decodo, template:
universal, ...}` — `Local` fetches first, escalating to `Provider` only on a
block or rate limit. `Save`/`Run` require at least one tier configured; pressing
`[ Run ]` with all three still `— not set` shows `✗ configure at least one of
Local/Proxy/Provider` instead of starting.

### Changing "Extract" reacts the same way

Moving `Extract` to `css fields` replaces the single-line field with an editable
list; `bare` and `raw` return the fetched body untouched (`bare` strips the
result envelope in item detail too, `raw` keeps it), and `markdown` adds a
markdown-conversion stage with no further fields:

```
│   Extract     › css fields               (bare / raw / markdown / css fields)        │
│     title         h1                                                                 │
│     price         .price@data-value                                                  │
│     + add field                                                                      │
```

### Crawl — same form model, different fields and defaults

```
┌─ Crawl ────────────────────────────────────────────────────────── esc: back to menu ─┐
│                                                                                      │
│      Starting URL                                                                    │
│      Local           — not set     (enter to configure)                              │
│      Proxy           — not set     (enter to configure)                              │
│      Provider         — not set     (enter to configure)                             │
│   › Depth           1           (0 = seeds only)                                    │
│      Max pages       100         (stop after this many pages)                        │
│      Scope           same-host   (same-host / same-domain / any)                     │
│      Sitemap         true        (also seed from /sitemap.xml)                       │
│      Robots          respect     (respect / ignore robots.txt)                       │
│      Extract         bare        (bare / raw / markdown / css fields)                │
│      Per-host rps    16          (0 = unlimited; default differs from Scrape)        │
│      Concurrency     8                                                               │
│      Retries         3                                                               │
│                                                                                      │
│   [ Save as job.yaml ]   [ Run ]                                                     │
│                                                                                      │
└──────────────────────────────────────────────────────────────────────────────────────┘
```

`Local`/`Proxy`/`Provider` behave exactly as in `Scrape`'s form above — same tier
screens, same automatic escalation once more than one is configured.

### Results — after `[ Run ]`

Pressing `[ Run ]` replaces the form with a scrollable list, one row per fetched
URL, status and a short detail preview alongside each. When a run configures more
than one tier, each row also names which tier actually served that URL — a row
reading `✓ 200 (proxy) title: ...` means that URL escalated to (or was served
directly by) `proxy`, not whatever tier the run started at. `enter` on a row opens
its full detail; `w` writes every result to disk; `esc` returns to the form. If
any URL came back blocked or rate-limited, a tip line appears above the list
suggesting a higher tier (Proxy or Provider) to auto-escalate past it next time.

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
│  › ✓ decodo    https://data.decodo.com/v1/scrape                                   │
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
| Local / Proxy / Provider (tier rows, each opens its own screen) | which tiers get configured determines `fetch.use` (the lowest configured tier) and `fetch.ladder` (`true` once 2+ are configured) |
| Profile (on the Local/Proxy tier screens) | `fetch.profile` (local/proxy only) |
| Proxy URL (on the Proxy tier screen) | `fetch.proxy` |
| Provider (on the Provider tier screen) | `fetch.name` |
| Template (on the Provider tier screen, multi-template providers only) | `fetch.template` |
| provider-declared fields (e.g. Headless / Geo, on the Provider tier screen) | `fetch.options.<name>` (provider only, per-manifest, per-template) |
| Extract (`bare`/`raw`) | no stage added; `bare` also strips the envelope in item detail (CLI equivalent: `--bare`) |
| Extract (`markdown`/`css fields`) | a stage in the pipeline (`markdown` / `css`) |
| Max pages (crawl only) | `source.crawl.maxPages` |
| Sitemap (crawl only) | `source.crawl.sitemap` |
| Robots (crawl only) | `source.crawl.robots` (`respect` / `ignore`) |
| Per-host rps (crawl only) | `limits.rps` |

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

**Check `empty` before trusting a successful run.** A challenge page pobear
doesn't recognize answers with a normal `200` and real HTML, so the fetch cannot
be classified as `[blocked]` the way a `403` can. What pobear can report is that the stages you asked for extracted nothing:

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
