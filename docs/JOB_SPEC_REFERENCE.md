# pobear job spec reference (spec)

This is a specification, not a record of a built system. It is the authoritative,
field-by-field rulebook for the job spec — the one document every front end (CLI
flags, a hand-written file, an SDK) produces and the engine is the only thing that
ever executes. `--print-spec`, on any command that supports it, shows this
document with every field still at its default omitted (§10); `--print-spec-verbose`
shows the same document with every default filled in, the way §11 spells out every
field's syntax. `fetch.options` carries provider-specific fields, each validated
against the schema the selected provider itself declares.

---

## 1. Top-level shape

```yaml
version: 1

source:
  urls: [https://a.test]         # exactly one of urls / crawl
  crawl: { seeds: [https://a.test] }

fetch:
  use: local

limits: {}

stages: []

sink:
  use: stdout
```

| Key | Required | §  |
|---|---|---|
| `version` | yes | §2 |
| `source` | yes | §3 |
| `fetch` | yes | §4 |
| `limits` | no (all fields default) | §5 |
| `stages` | no (default `[]`) | §6 |
| `sink` | no (default `{use: stdout}`) | §7 |

A spec is written as YAML or JSON — one schema, two encodings. Every field name
below is exact and case-sensitive; an unrecognized key anywhere in the document is
a validation error (§8), not a silently-ignored typo.

---

## 2. `version`

| Rule |
|---|
| Must be the integer `1`. Absent, or any other value, is rejected. |

---

## 3. `source`

**Rule:** exactly one of `urls` or `crawl` must be set. Both set, or neither set, is
rejected at validation time.

### 3.1 `source.urls`

| Type | Rule |
|---|---|
| list of strings | non-empty; every entry an absolute `http`/`https` URL |

### 3.2 `source.crawl`

| Field | Type | Default | Rule |
|---|---|---|---|
| `seeds` | `[]string` | — (required) | non-empty; every entry an absolute `http`/`https` URL |
| `depth` | `int` | `0` in a bare spec; the CLI's `--depth` defaults it to `1` | `0` = seeds only, no link-following; negative rejected |
| `maxPages` | `int` | `100` (CLI default; no spec-level default) | must be `> 0` |
| `maxDuration` | duration string | `""` (no limit) | standard duration syntax (§9) or empty |
| `scope` | enum: `same-host` \| `same-domain` \| `any` | `same-host` | anything else rejected |
| `include` | `[]glob` | `[]` | doublestar glob syntax (`**` matches across `/`) |
| `exclude` | `[]glob` | `[]` | same syntax; **wins over `include`** when both match a URL |
| `sitemap` | `bool` | `false` | also seed the frontier from `/sitemap.xml` |
| `robots` | enum: `respect` \| `ignore` | `respect` | anything else rejected |

`depth`, `maxPages`, and `maxDuration` are independent stopping conditions — a crawl
stops the moment any one of them is hit.

**Dedup:** a crawl never fetches or queues the same URL twice — always on, not a
field. A `seed`, a `sitemap`-discovered URL, and a discovered link are all claimed
against the same run-scoped set the moment each is found, before it's ever added
to the frontier; a second sighting of an already-claimed URL is discarded
outright (not counted as `skipped` — it was never a missed fetch, since its
canonical form already ran or is already queued to). Two URLs are the same if
they're identical once the fragment is dropped and the query string is
canonicalized (both key order and, within a key, its own repeated values,
sorted) — `page#a` vs `page#b` and `?a=1&b=2` vs `?b=2&a=1` are the same URL.
No other normalization applies: scheme/host case, default ports, and trailing
slashes are compared exactly as given.

**`scope: same-host` and `www.`:** a leading `www.` is stripped from the host
before comparison, so `example.com` and `www.example.com` are treated as the
same host regardless of which one is the seed — this is the one normalization
`same-host` applies; any other subdomain (`blog.example.com`) is still a
different host under `same-host` (use `same-domain` for that).

**Link discovery:** every `<a href>` in a fetched HTML body is a candidate link,
resolved to an absolute URL against the page's own final URL (after redirects) —
an in-page `<base href>` override is not honored. Anything that isn't a
`http`/`https` URL once resolved (`mailto:`, `tel:`, `javascript:`, an unparsable
`href`) is silently dropped; it was never a crawlable candidate.

**What the frontier filters apply to:** `scope`, `include`/`exclude` and `robots`
are evaluated on every candidate before it is fetched, and again on the URL a
fetch actually landed on when a redirect moved it — a redirect into a disallowed
or out-of-scope URL is neither emitted nor followed. Seeds are the one exemption:
a URL the user named explicitly is fetched as asked, and keeps that exemption
through a redirect. Sitemap-sourced URLs are **not** seeds — the site chose them,
so every filter applies to them exactly as it does to a discovered link.

**`robots`/`sitemap` fetching:** both `<origin>/robots.txt` and
`<origin>/sitemap.xml` are always fetched via a plain, direct HTTP request,
regardless of `fetch.use` — these files are meant to be openly fetchable by any
crawler, so there's no benefit to routing them through a configured
proxy/provider, and doing so is often incoherent anyway (a provider's `template`
is bound to a specific request shape, not a generic file fetch). Robots rules
are always matched against the wildcard `User-agent: *` group. A `robots.txt`
that 404s (or any 4xx) means no restrictions apply; any other failure (network
error, another status, or a body that fails to parse) fails closed — every URL
under that origin is treated as disallowed for the rest of the crawl, not
allowed through. `sitemap.xml` follows one level of `<sitemapindex>` recursion
(an index pointing to per-section sitemaps); gzip-compressed sitemaps aren't
supported.

---

## 4. `fetch`

| Field | Type | Valid on tier | Default | Rule |
|---|---|---|---|---|
| `use` | enum: `local` \| `proxy` \| `provider` | all | — (required) | selects the tier |
| `name` | string | `provider` only | — | must name an installed, loaded provider manifest |
| `template` | string | `provider` only | the manifest's `default: true` template, if any | selects one of `name`'s declared templates; required if the manifest declares 2+ templates with none marked `default` |
| `proxy` | string (URL) | `proxy` only | — | any `http`/`https`/`socks5`/`socks5h` proxy URL |
| `profile` | enum: `chrome` \| `firefox` \| `safari` | `local`, `proxy` | `chrome` | TLS fingerprint preset |
| `options` | `map[string]any` | `provider` only | `{}` | validated against the selected template's declared options (or the manifest's top-level ones, if it has no templates) |
| `timeout` | duration string | all | `30s` | per-request timeout |
| `noRedirects` | `bool` | all | `false` | `true` = a `3xx` is the final result; its `Location` is never followed, and in crawl mode never added to the frontier |
| `headers` | `map[string]string` | all | `{}` | extra request headers, sent as-is |

**Rule:** setting a field on a tier that doesn't support it is a validation error
naming the field and the tier — never a silent no-op. This includes `options`: it
requires `use: provider`, and every key in it must be declared by `name`'s manifest
with a compatible type and value.

**`profile` only spoofs the TLS ClientHello, nothing else:** it doesn't touch
`User-Agent` or any other header — a browser-shaped TLS fingerprint paired
with no `User-Agent` (or a mismatched one) is itself a detectable,
inconsistent signal, so set `headers.User-Agent` yourself (§ `headers` above)
if you want one to match. Each profile currently pins one specific browser
version at the TLS layer — Chrome 133, Firefox 120, Safari 16.0 — matching
whatever version `github.com/refraction-networking/utls`'s
`HelloChrome_Auto`/`HelloFirefox_Auto`/`HelloSafari_Auto` presets build a
ClientHello for at the time this was implemented
(`services/core/internal/fetch/tiers/fingerprint.go`); a future `utls` upgrade
moving `_Auto` to a newer version isn't automatically reflected anywhere else.

---

## 5. `limits`

| Field | Type | Default | Rule |
|---|---|---|---|
| `concurrency` | `int` | `8` | `> 0` |
| `rps` | `int` | `0` (scrape CLI) / `2` (crawl CLI); `0` in a bare spec | `0` = unlimited; negative rejected; caps total requests/sec across every host combined, not per host |
| `retries` | `int` | `3` | `>= 1` (the first attempt counts) |
| `backoff` | enum | `exponential` (CLI default) | only `exponential` is currently a valid value — anything else rejected, not silently substituted |
| `pluginConcurrency` | `int` | `0` | `__rpc` mode only; ignored elsewhere |
| `pluginTimeout` | duration string | `30s` | `__rpc` mode only; ignored elsewhere |

`concurrency` genuinely bounds how many fetches run at once, for both a plain
URL list and a crawl's frontier. For a crawl specifically, `source.crawl.maxPages`
is an exact cap on the number of fetches dispatched, regardless of
`concurrency`: a page is admitted against the budget atomically, before its
fetch starts, so the total dispatched can never exceed `maxPages` — only the
tiny admission check is serialized, not the fetch itself, so this costs
nothing in concurrency.

`__rpc` mode, and everything gated behind it (`pluginConcurrency`, `pluginTimeout`,
the `plugin` extraction method below), is deferred — not part of the v1 release. `pkg/plugin`
has been removed from the codebase; these fields stay documented as the intended
shape for when `__rpc`/SDK support lands.

---

## 6. `stages`

An ordered list of `{use: <name>, ...config}`. Applied in order; each stage reads
the fetched body and writes into the item's `data` map — no stage mutates the body,
so stages are safe to reason about independently of order. Two stages are never
allowed to write the same `data` key (see **Rules** below), so there is no
order-dependent overwrite to reason about either.

| `use` | Config | Writes |
|---|---|---|
| `raw` | — | `data.raw`: the untouched body |
| `markdown` | — | `data.markdown`: HTML converted to Markdown |
| `css` | `fields: {name: "selector"}` or `{name: "selector@attr"}` | `data[name]` per field — always a list of every matching element's value, in document order; `[]` if nothing matched |
| `plugin` | `id: "<registered id>"` | whatever the host callback returns |

**Rules:**
- `use` must be one of the four names above; anything else is rejected at load time.
- A `plugin` stage is rejected outside `__rpc` mode (there is no reverse channel to
  answer it in plain CLI mode) — a clear error, not a hang. `__rpc` mode is
  deferred (see §5), so in practice `plugin` is unreachable in v1.
- Two stages writing the same `data` key is rejected at load time, naming both
  stages involved by position — the same as any other invalid spec, never a
  silent overwrite and never something that reaches a running job. This is
  checkable statically for `raw`/`markdown`/`css` (their output keys are fully
  determined by their own config: a fixed name, or `fields`'s own keys), so it
  never depends on what the fetched body actually contains. A `plugin` stage's
  output key is decided by the host callback at runtime, so it's exempt from
  this check — moot in practice today since `plugin` is unreachable outside
  `__rpc` mode (deferred, see §5).

A textual response body is decoded to UTF-8 using whatever charset the response
declares — the `Content-Type` header, a BOM, or a `<meta charset>` tag — so a
page served as, say, ISO-8859-1 reads correctly in `body` and in every extracted
value. An unrecognised charset label falls back to the HTML default of
windows-1252, matching browser behaviour. Bodies that aren't textual are left as
the bytes the server sent, since no charset applies and sniffing them would
change the content.

Every fetched item already carries the untouched body in a top-level `body` field
(see `CLI_REFERENCE.md`'s examples), independent of `stages`. An empty `stages: []`
still gives you that field — `raw` doesn't fetch anything extra, it just copies the
same content into `data.raw` too, which is what makes it eligible for `--bare` and
lets it sit alongside other fields (`css`, `markdown`) in the same `data` map.

---

## 7. `sink`

| Field | Type | Default | Rule |
|---|---|---|---|
| `use` | enum: `stdout` \| `ndjson` \| `json` \| `yaml` \| `stream` | `stdout` | anything else rejected |
| `path` | string | — | **required** when `use` is `ndjson`/`json`/`yaml`; **rejected** when `use` is `stdout`/`stream` |

- `stdout` streams NDJSON to stdout.
- `ndjson` streams NDJSON to `path` as items complete.
- `json` buffers every item and writes one JSON array to `path` on completion, so a
  reader never observes a half-written array.
- `yaml` buffers every item and writes one YAML sequence to `path` on completion —
  same buffer-until-complete shape as `json`, same field names, just YAML instead
  of JSON on disk.
- `stream` pushes items to the RPC streamer instead of a local file/stdout. Like the
  `plugin` extraction method (§6), it's only meaningful when there's a wire connection to push
  over: a `sink.use: stream` spec is rejected outside `__rpc` mode, a clear error
  rather than a hang.
- A closed sink never silently accepts more items: calling `Write` after `Close`
  returns a non-nil error, on all sink kinds.
- `Close` with zero items written still produces valid output: `json`/`yaml` write
  an empty array/sequence (`[]`, not `null`), and `ndjson` still creates `path` as
  an empty (0-byte) file — a downstream reader never sees a missing file just
  because nothing was fetched.

---

## 8. Validation — the full list

Every spec is checked before anything runs. All of the following are load-time
errors, naming the offending field, not runtime surprises three hundred pages into
a crawl:

- Unknown keys anywhere in the document (strict decoding).
- A multi-document YAML file.
- `version` missing or not `1`.
- `source`: neither or both of `urls`/`crawl` set; empty `urls`/`seeds`; a
  non-absolute or non-`http(s)` URL anywhere in either.
- `source.crawl`: negative `depth`/`maxPages`; `maxDuration` not a valid duration;
  `scope`/`robots` outside their enums; malformed `include`/`exclude` globs.
- `fetch`: a field set for a tier that doesn't support it (§4); `profile`/`use`
  outside their enums; `timeout` not a valid duration.
- `fetch.template`: set without `use: provider`; names a template `name`'s
  manifest doesn't declare; omitted when the manifest has 2+ templates and none
  is marked `default`.
- `fetch.options`: set without `use: provider`; a key the selected template (or
  the manifest's top-level options, if it has no templates) doesn't declare; a
  value of the wrong type; an `enum` value outside `values`; a `string` value
  that doesn't match `pattern`; an `int` value outside `min`/`max`.
- `limits`: any field `<= 0` where the rule requires `> 0`; `backoff` not
  `exponential`; negative `rps`.
- `stages[].use` not one of the four known extraction-method names; a stage using
  `plugin` outside `__rpc` mode; a stage using `css` with no `fields`.
- `sink.use` outside its enum; `path` missing when required, or present when
  forbidden; `sink.use: stream` outside `__rpc` mode.

---

## 9. Duration and glob syntax

- **Durations** (`timeout`, `maxDuration`, `pluginTimeout`) use Go's
  `time.ParseDuration` syntax: an optional sign, then one or more decimal numbers
  each followed by a unit — `ms`, `s`, `m`, `h` — e.g. `"500ms"`, `"30s"`, `"10m"`,
  `"1h30m"`. An empty string means "no limit" wherever that's meaningful
  (`maxDuration`); it is never valid for `timeout`.
- **Globs** (`include`, `exclude`) match against the **full URL** — scheme, host,
  path, and query string together as one string — not just the path. `**` matches
  across `/` (e.g. `**/blog/**` matches any depth under `/blog/` on any host), and
  standard single-segment `*` (anything but `/`) and `?` (exactly one character)
  otherwise. Because `?` is also glob syntax, matching a literal `?` (a URL's query
  separator) requires escaping it as `\?` — e.g. `**\?*sessionid=*` excludes any URL
  with a `sessionid` query parameter, at any path.

---

## 10. Minimal valid spec

```yaml
version: 1
source: { urls: [https://example.com] }
fetch: { use: local }
```

Every other field takes its default. This is exactly what
`pobear scrape https://example.com --print-spec` produces before any optional flag
is added — `--print-spec-verbose` on the same command would spell out every one of
those defaults instead of omitting them.

---

## 11. Full annotated spec

```yaml
version: 1

source:
  crawl:
    seeds: [https://a.test]
    depth: 2
    maxPages: 100
    maxDuration: 30m
    scope: same-host
    include: ["**/blog/**"]
    exclude: ["**/tag/**"]
    sitemap: false
    robots: respect

fetch:
  use: provider
  name: decodo
  template: universal
  options: { render: true, geo: us }
  timeout: 30s
  noRedirects: false
  headers: { "X-Custom": "1" }

limits:
  concurrency: 8
  rps: 2
  retries: 3
  backoff: exponential
  pluginConcurrency: 4
  pluginTimeout: 30s

stages:
  - use: markdown
  - use: css
    fields: { title: "h1", price: ".price@text" }

sink:
  use: ndjson
  path: out.ndjson
```

---

## 12. How front ends produce this document

| Front end | Produces a spec by |
|---|---|
| CLI (`scrape`/`crawl`) | parsing flags into this document |
| `pobear run <job.yaml\|->` | parsing a file, or stdin when given `-` |
| Interactive mode (`pobear wizard`) | a reactive form over these same fields |

`--print-spec`, wherever it's available, prints exactly this document (defaults
omitted) instead of running it — the same document any of the above would otherwise
hand to the engine. `--print-spec-verbose` prints the identical document with every
default filled in; both are equally valid input to `pobear run`.
