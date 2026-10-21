# pobear provider reference (spec)

This is a specification, not a record of a built system. It is the authoritative
rulebook for provider manifests — the YAML files that describe how to call one
scraping API, with no code involved anywhere. Nothing in the engine is specific to
any provider; Decodo's own manifest is validated and executed by the same generic
path every third-party manifest is. A job spec binds to a manifest through
`fetch.name` (which one) and `fetch.options` (which of its declared fields to set).

---

## 1. Top-level shape

```yaml
name: acme
endpoint: https://api.acme.test/v1/scrape
method: POST
auth: { type: bearer, credential: token }
options: []
request: { body: {}, query: {} }
response: { body: "$.html" }
errors: []
```

| Key | Required | §  |
|---|---|---|
| `name` | yes | §2 |
| `endpoint` | yes | §3 |
| `method` | no (default `POST`) | §4 |
| `auth` | yes | §5 |
| `headers` | no | §5a |
| `options` | no (default `[]`) | §6 |
| `request` | yes | §7 |
| `response` | yes (`body` required within it) | §9 |
| `errors` | no (default `[]`) | §10 |
| `templates` | no; **mutually exclusive with top-level `options`/`request`** | §11 |

A manifest either declares flat top-level `options`/`request` as shown above (a
single implicit mode), or declares `templates` (§11) — multiple named modes, each
with its own `options`/`request`, for a provider whose fields depend on which kind
of page you're asking it to scrape. Never both.

---

## 2. `name`

| Rule |
|---|
| Required string, matching `[a-z0-9_]+`. Must be unique across every manifest `pobear` loads — two manifests sharing a name is a load-time error, not "last one wins." |

This is the value passed to `--provider <name>` / `fetch.name`, and it's the `<NAME>`
segment of the credential environment variable (§5). The charset restriction exists
because that segment is joined unmodified into a shell environment variable name —
`pkg/credentials.EnvVar` does no normalization, so a name outside `[a-z0-9_]+` would
produce an env var name that isn't legally exportable in most shells.

---

## 3. `endpoint`

| Rule |
|---|
| Required string. Must be an absolute `http`/`https` URL. |

A `templates` entry (§11) may override this for itself; every template that
doesn't set its own inherits this one.

---

## 4. `method`

| Rule |
|---|
| Optional string. Defaults to `POST`. |

A `templates` entry (§11) may override this for itself the same way it can
`endpoint`.

---

## 5. `auth`

| Field | Type | Required | Rule |
|---|---|---|---|
| `type` | enum: `basic` \| `bearer` \| `header` \| `query` | yes | anything else rejected |
| `credential` | string | yes | the env-var suffix (below); not the secret itself |
| `name` | string | only for `type: header` / `type: query` | the header name or query param name the credential is sent as |

**Credential env var rule:** the actual secret is never stored in a manifest or a
job spec. It's read from `POBEAR_PROVIDER_<NAME>_<CREDENTIAL>`, both segments
uppercased — `name: acme` + `credential: token` → `POBEAR_PROVIDER_ACME_TOKEN`.
`pobear auth <name>` prompts for it (not echoed) and stores it for every subsequent
run; `pobear scrape --provider <name>` fails with a clear auth error if it's unset.
The on-disk credentials store `pobear auth` writes to is created with `0o600`
permissions, since it holds plaintext secrets.

**`type: basic`** stores one secret like every other type — the already
base64-encoded `username:password` credential (what a user copies straight out
of most APIs' own docs/consoles), entered as one answer to the `pobear auth
<name>` prompt and sent verbatim as `Authorization: Basic <secret>`. pobear
never encodes or decodes it itself, and there's no separate username/password
pair at the manifest level.

**`type: query`** sends the credential as a plain query parameter, which means it
ends up in the request URL — visible in proxy logs, server access logs, and browser
history if that URL is ever opened by hand. Prefer `bearer`/`header` when a
provider supports them; use `query` only when the provider's API leaves no other
option.

---

## 5a. `headers`

| Rule |
|---|
| Optional mapping of header name to a static string value, sent on every request to this provider. Values are not templates. Precedence, lowest to highest: manifest `headers`, then the job's `fetch.headers` (`--header`), then `auth` — so a manifest header can be overridden per job, and neither can replace the credential. |

Decodo's manifest uses it to send `x-integration: oss-pobear`, which attributes
pobear traffic on the provider's side.

---

## 6. `options` — the fields this provider accepts

A list of fields this provider's API accepts beyond the URL itself — declared as
data, with no executable logic, ever. This is the *only* place a manifest expresses
validation: what a field is called, its type, and simple per-field constraints.
This same shape is used whether it's declared here, at the manifest's top level,
or inside one entry of `templates` (§11) — a template's `options` list follows
every rule on this page identically, just scoped to that template.

```yaml
options:
  - name: render
    type: bool
    default: false
  - name: geo
    type: string
    pattern: '^[a-z]{2}$'
  - name: device
    type: enum
    values: [desktop, mobile]
    default: desktop
  - name: maxWait
    type: int
    min: 0
    max: 60000
```

| Field | Applies to | Required | Rule |
|---|---|---|---|
| `name` | all | yes | unique within this manifest; must not be `url` (reserved) |
| `type` | all | yes | one of `bool` \| `string` \| `int` \| `enum` |
| `default` | all | no | must satisfy this option's own constraints if set |
| `pattern` | `string` | no | a valid regular expression |
| `min`, `max` | `int` | no | `min <= max` if both set |
| `values` | `enum` | yes | non-empty; `default` (if set) must be one of them |

`render` and `geo` are not special-cased by the engine — Decodo's manifest declares
them exactly like `device` above. `--render`/`--geo` on the CLI are sugar for the
generic `--opt render=true --opt geo=us`; a manifest that omits an option simply
doesn't support it, and asking for it anyway is rejected the same way an
out-of-range value is (§12). A provider whose API names the param differently can
still support the sugar by declaring the option and mapping it onto its own param
in a request template (§8).

**Known gap:** constraints are per-field only. A manifest cannot express "`geo` is
required when `render` is true," or any rule spanning more than one field — that
needs a real callback into provider logic, which is exactly what the (deferred)
plugin channel is for, not a bigger constraint language bolted onto `options`.

---

## 7. `request`

| Field | Type | Rule |
|---|---|---|
| `body` | `map[string]any` | string values are Go templates (§8); non-string values (bool, number, nested object) pass through as real JSON, untouched |
| `query` | `map[string]scalar` | string values are Go templates (§8); values must be scalars — no lists or maps in a query string |

**Rule:** a template that renders to an empty string is **dropped** from the request
entirely, not sent as `""` — most provider APIs reject an empty enum/param, and
absent is not the same as empty.

Like `options`, this shape is identical whether declared here or inside one entry
of `templates` (§11).

---

## 8. Template variables

Exactly two things are available inside a `request.body`/`request.query` string
template — nothing else about the job (no headers, no credentials, no other item
fields):

| Variable | Type | Meaning |
|---|---|---|
| `.URL` | string | the item's URL |
| `.Options.<name>` | per that option's declared `type` | one entry per option this manifest declared (§6); a `bool` renders as `true`/`false`, an `int` as its decimal value |

```gotemplate
js: "{{ if .Options.render }}true{{ end }}"
device: "{{ .Options.device }}"
```

---

## 9. `response`

| Field | Type | Required | Rule |
|---|---|---|---|
| `status` | JSONPath | no | maps to the response's logical status; falls back to the actual HTTP status of the response if unset |
| `body` | JSONPath | **yes** | the extracted page content |
| `finalUrl` | JSONPath | no | falls back to the request URL if unset |

JSONPath syntax is validated at load time, before any request is ever sent.

Because `body` is required, a response whose declared `body` path matches
**nothing** is a broken provider contract, not an empty page: the fetch fails
with fault kind `provider` and the error carries a snippet of the actual
response. A path that matches an *empty value* is a legitimately empty page and
succeeds. The status (§10) is classified first, so an error response that
carries none of the declared fields — commonly HTML or plain text — is still
reported by its status rather than as a missing body.

This resolved status — the extracted logical status, or the transport-level HTTP
status if `status` is unset — is the value `errors[].status` (§10) matches against.
It exists for providers that don't use HTTP status codes to report outcome at all
(e.g. always answering `200` with the real result embedded in the JSON body); for
a provider that does use real HTTP status codes, leaving `status` unset makes the
two the same thing.

---

## 10. `errors`

A list mapping this provider's resolved status (§9 — `response.status`, or the
real HTTP status if that's unset) to pobear's fault taxonomy:

```yaml
errors:
  - status: 401
    kind: auth
  - status: 429
    kind: rateLimit
```

| Field | Rule |
|---|---|
| `status` | must be a real HTTP status code |
| `kind` | one of the **seven fetch-outcome kinds**: `auth`, `network`, `timeout`, `http`, `blocked`, `rateLimit`, `provider`. `spec`, `parse`, and `plugin` are not assignable here — `spec` is a job-level fault a provider has no business declaring (it would let a manifest unilaterally abort every job that uses it), and the other two aren't fetch outcomes at all. |

Any resolved status not listed falls through to the engine's default HTTP-status
classification.

---

## 11. `templates` — multiple named modes in one manifest

Some providers expose one endpoint that behaves differently depending on a field
you send it — Oxylabs/Decodo's `source: universal` vs. `source: amazon_product`,
or SerpApi's `engine: google` vs. `engine: amazon`. Others expose a genuinely
different endpoint per site or data type instead (ScraperAPI's dedicated Amazon,
Google, and Walmart paths). `templates` covers both shapes: a named list of modes,
each with its own `options` (§6) and `request` (§7), and each free to override
this manifest's `endpoint`/`method` or inherit them unchanged.

```yaml
templates:
  - name: universal
    default: true
    options:
      - name: render
        type: bool
        default: false
      - name: geo
        type: string
        pattern: '^[a-z]{2}$'
    request:
      body:
        source: universal
        url: "{{ .URL }}"
        render: "{{ .Options.render }}"
        geo: "{{ .Options.geo }}"

  - name: amazon_product
    endpoint: https://api.acme.test/v1/amazon   # overrides this manifest's endpoint
    options:
      - name: domain
        type: enum
        values: [com, co.uk, de]
        default: com
    request:
      body:
        query: "{{ .URL }}"
        domain: "{{ .Options.domain }}"
```

| Field | Type | Required | Rule |
|---|---|---|---|
| `name` | string | yes | unique within this manifest |
| `default` | `bool` | no | at most one template may set this `true` |
| `endpoint` | string | no | overrides this manifest's `endpoint` (§3) for this template only; same absolute-URL rule |
| `method` | string | no | overrides this manifest's `method` (§4) for this template only |
| `options` | list | no | same shape and rules as §6, scoped to this template |
| `request` | object | yes | same shape and rules as §7, scoped to this template |

**Selecting a template:** the job spec's `fetch.template` names one. If a manifest
has exactly one template, it's used regardless of `default`. If it has more than
one and none is marked `default: true`, `fetch.template` is required — omitting it
is a validation error, not a guess at which one you meant.

**Mutually exclusive with top-level `options`/`request`:** a manifest sets one or
the other, never both. A manifest with no `templates` key is unaffected by any of
this — it's exactly the single-implicit-mode case documented in §6/§7.

Zyte's real shape — several mutually-exclusive booleans (`product: true` /
`article: true` / …) rather than one string field — needs no special case here:
each becomes its own template name, and that template's own `request.body` sets
the one boolean that corresponds to it.

---

## 12. Validation — the full list

Everything checkable is checked when `pobear` starts, not on the hundredth request —
because a manifest is data written by a third party, not code this project wrote
and already trusts:

- `endpoint` is an absolute `http`/`https` URL.
- `auth.type` is one of the four supported kinds, and `credential`/`name` are
  present where that kind requires them.
- `options`: names are unique and not `url`; `type` is one of the four kinds;
  `enum` has a non-empty `values` and an in-range `default` if set; `string.pattern`
  compiles as a regex; `int.min <= int.max`.
- `request.body`/`request.query` template syntax is valid Go template syntax.
- `request.body`/`request.query` values are JSON-encodable and non-null.
- `request.query` values are scalars (no lists/maps).
- `response.status`/`response.body`/`response.finalUrl` are valid JSONPath syntax.
- `response.body` is present.
- `errors[].kind` is one of the seven fetch-outcome kinds (§10); `errors[].status`
  is a real HTTP status.
- `templates` and top-level `options`/`request` are never both set.
- `templates` (if set): at least one entry; names unique; at most one
  `default: true`; each entry's `endpoint` (if set) is an absolute `http`/`https`
  URL; each entry's `options`/`request` follow the same rules as §6/§7.
- No two loaded manifests share a `name`, and `name` matches `[a-z0-9_]+` (§2).

A manifest failing any of these fails at `pobear` startup with a message naming the
manifest and the field — it never partially loads.

---

## 13. Installing and using one

Decodo ships embedded in the `pobear` binary itself — it's available with no
setup beyond `pobear auth decodo`. Its manifest is generated, not hand-written:
`go generate ./providers` fetches the latest Decodo target schema and emits one template per target
that takes a `url`. Targets keyed on a query, prompt or product ID are omitted,
since every pobear item is a URL, as are targets pobear deliberately doesn't offer
(currently Reddit's). Parameters that are async-only (`callback_url`),
change the response shape (`markdown`, `xhr`), or aren't scalars (`cookies`,
`headers`, …) are not exposed as options. It targets the Web Data API
(`data.decodo.com`, `bearer` auth with an API key).

Any other provider is installed by dropping its manifest into pobear's
providers directory: `$POBEAR_PROVIDERS_DIR` if set, otherwise
`~/.pobear/providers/` (next to the credentials store). The current working
directory is never searched, so an unrelated `providers/` folder in whatever
project pobear runs from can't break or shadow anything.

```bash
# 1. Drop the file where pobear looks for manifests
mkdir -p ~/.pobear/providers
cp acme.yaml ~/.pobear/providers/          # or $POBEAR_PROVIDERS_DIR

# 2. Confirm it loaded and see what it declares
pobear providers

# 3. Store its credential (prompts, not echoed)
pobear auth acme

# 4. Use it
pobear scrape https://example.com --provider acme --opt device=mobile
```

`pobear providers` always lists the embedded manifests too, merged with
whatever's in the providers directory — a manifest on disk with the same
`name` overrides the embedded one, so a customized `decodo.yaml` can still
replace the bundled definition if needed. The wizard's `Manage providers`
menu item covers the same ground interactively — browse what's loaded, view
a manifest's full schema, remove an on-disk one (the embedded copy, if any,
stays available after removing an override), and set or replace a stored
credential.

---

## 14. Minimal valid manifest

```yaml
name: minimal
endpoint: https://api.minimal.test/scrape
auth: { type: bearer, credential: token }
request:
  body: { target: "{{ .URL }}" }
response:
  body: "$.html"
```

`method` defaults to `POST`, `options`/`errors` default to empty, and `response`
falls back to the real HTTP status and the request URL — this is the smallest
manifest that can actually serve a request.
