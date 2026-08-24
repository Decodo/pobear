# pobear job spec reference (spec)

This is a specification, not a record of a built system. It is the authoritative,
field-by-field rulebook for the job spec — the one document every front end (CLI
flags, a hand-written file, an SDK) produces and the engine is the only thing that
ever executes. `--print-spec`, on any command that supports it, shows exactly the
document described here. `fetch.options` carries provider-specific fields, each
validated against the schema the selected provider itself declares.

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

---

## 4. `fetch`

| Field | Type | Valid on tier | Default | Rule |
|---|---|---|---|---|
| `use` | enum: `local` \| `proxy` \| `provider` | all | — (required) | selects the tier |
| `name` | string | `provider` only | — | must name an installed, loaded provider manifest |
| `template` | string | `provider` only | the manifest's `default: true` template, if any | selects one of `name`'s declared templates; required if the manifest declares 2+ templates with none marked `default` |
| `proxy` | string (URL) | `proxy` only | — | any `http`/`https`/`socks5` proxy URL |
| `profile` | enum: `chrome` \| `firefox` \| `safari` | `local`, `proxy` | `chrome` | TLS fingerprint preset |
| `options` | `map[string]any` | `provider` only | `{}` | validated against the selected template's declared options (or the manifest's top-level ones, if it has no templates) |
| `timeout` | duration string | all | `30s` | per-request timeout |
| `noRedirects` | `bool` | all | `false` | `true` = a `3xx` is the final result; its `Location` is never followed, and in crawl mode never added to the frontier |
| `headers` | `map[string]string` | all | `{}` | extra request headers, sent as-is |

**Rule:** setting a field on a tier that doesn't support it is a validation error
naming the field and the tier — never a silent no-op. This includes `options`: it
requires `use: provider`, and every key in it must be declared by `name`'s manifest
with a compatible type and value.

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

---

## 6. `stages`

An ordered list of `{use: <name>, ...config}`. Applied in order; each stage reads
the fetched body and writes into the item's `data` map — no stage mutates the body,
so stages are safe to reason about independently of order (except for `data` key
collisions, below).

| `use` | Config | Writes |
|---|---|---|
| `raw` | — | `data.raw`: the untouched body |
| `markdown` | — | `data.markdown`: HTML converted to Markdown |
| `css` | `fields: {name: "selector"}` or `{name: "selector@attr"}` | `data[name]` per field; `null` if nothing matched |
| `plugin` | `id: "<registered id>"` | whatever the host callback returns |

**Rules:**
- `use` must be one of the four names above; anything else is rejected at load time.
- A `plugin` stage is rejected outside `__rpc` mode (there is no reverse channel to
  answer it in plain CLI mode) — a clear error, not a hang.
- If two stages write the same `data` key, the second write wins and a warning is
  printed naming both stages by position — never a silent overwrite.

---

## 7. `sink`

| Field | Type | Default | Rule |
|---|---|---|---|
| `use` | enum: `stdout` \| `ndjson` \| `json` | `stdout` | anything else rejected |
| `path` | string | — | **required** when `use` is `ndjson`/`json`; **rejected** when `use` is `stdout` |

- `stdout` streams NDJSON to stdout.
- `ndjson` streams NDJSON to `path` as items complete.
- `json` buffers every item and writes one JSON array to `path` on completion, so a
  reader never observes a half-written array.

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
- `stages[].use` not one of the four known stage names; a `plugin` stage outside
  `__rpc` mode; a `css` stage with no `fields`.
- `sink.use` outside its enum; `path` missing when required, or present when
  forbidden.

---

## 9. Duration and glob syntax

- **Durations** (`timeout`, `maxDuration`, `pluginTimeout`) use Go's
  `time.ParseDuration` syntax: an optional sign, then one or more decimal numbers
  each followed by a unit — `ms`, `s`, `m`, `h` — e.g. `"500ms"`, `"30s"`, `"10m"`,
  `"1h30m"`. An empty string means "no limit" wherever that's meaningful
  (`maxDuration`); it is never valid for `timeout`.
- **Globs** (`include`, `exclude`) support `**` to match across path segments (e.g.
  `/blog/**` matches any depth under `/blog/`), and standard single-segment `*`/`?`
  otherwise.

---

## 10. Minimal valid spec

```yaml
version: 1
source: { urls: [https://example.com] }
fetch: { use: local }
```

Every other field takes its default. This is exactly what
`pobear scrape https://example.com --print-spec` produces before any optional flag
is added.

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
    include: ["/blog/**"]
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

`--print-spec`, wherever it's available, prints exactly this document instead of
running it — the same document any of the above would otherwise hand to the engine.
