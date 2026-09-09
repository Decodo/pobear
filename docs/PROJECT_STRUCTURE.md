## Project architecture
- Core - Go project with all scraping and crawling logic, this part is what is doing the heavy lifting, managing requests, throttling, concurrency, dedupe, understanding job specs and executing them - in simple terms “business logic” is here
- CLI - terminal tool interface to use Core, this part prints pretty “UI” in the terminal - “frontend” for the tool.
- RPC layer - Since we want to use this same logic for any SDK we might create, we’d want a way to communicate between specific programming language and core, this part should not be seen by the user. It’ll work as a separate CLI that will accept a slightly modified job spec, stream the progress and then output final response.
- SDKs - Bindings, any number of SDKs with desired languages, a small interface to interact with RPC layer.

### Project structure
```
/cmd
  /core
    main.go                       // builds the single `pobear` binary: wires cli, rpc and core services together (incl. the hidden `__rpc` subcommand)

/pkg
  /jobspec                        // job spec document (JOB_SPEC_REFERENCE.md): types + validation, shared by every front end and by Core
    types.go                       // the Spec type tree (§1–§7)
    parse.go                       // Parse(data, mode): strict decode + §8 validation, resolving every optional field to its default; ValidationError names the offending field
    /testdata/validation           // one fixture per JOB_SPEC_REFERENCE.md §8 rule (input + expected outcome), walked by a single generic test
  /provider                       // provider manifest document (PROVIDER_REFERENCE.md): types + validation, shared by providerator (CLI) and the `provider` fetch tier (Core)
    template.go                    // renders a template's request.body/request.query Go-templates against .URL/.Options.<name>, and resolves which of a manifest's declared templates applies (fetch.template) — its own file since template resolution is complex enough to reason about separately from the rest of the manifest shape
  /credentials                    // shared provider-credential store (POBEAR_PROVIDER_<NAME>_<CREDENTIAL> / ~/.pobear/credentials) — written by cli/internal/providerator's provider-auth.go, read by core's provider fetch tier at request time
  /fault                          // the fault-kind taxonomy (auth/network/timeout/http/blocked/rateLimit/provider + job-level spec/parse/plugin) — what a manifest's errors[] classifies a response into, and what pkg/exitcode's classification keys off of. It does NOT gate fetcher's same-tier retry eligibility in v1: every failed attempt is retried up to limits.retries, regardless of fault.Kind
  /exitcode                       // maps a finished job's outcome (via pkg/fault) to the CLI's process exit code (0/1/3/4/5; 2 is reserved for a Go panic)
  /plugin                         // DEFERRED, not in v1 — removed from the codebase along with __rpc/SDK support (JOB_SPEC_REFERENCE.md §5). Was: the `plugin` extraction method's channel, kept global since it's meant for whatever plugins we allow creating, not specific to one extractor implementation. Reintroduce alongside services/rpc when that lands: a pluginer.go (entry point core/internal/stage/extractors/plugin.go calls, resolving a registered id and dispatching the callback, enforcing limits.pluginConcurrency/limits.pluginTimeout), a registry.go (tracks which plugin ids are currently registered by the connected RPC host), and a caller.go (the interface pluginer dispatches through, implemented by services/rpc/internal/streamer and injected, since pkg can't import services/rpc/internal directly).

/services
  /core                          // "Core" container in the diagram — the one engine that executes a spec, regardless of which front end produced it
    service.go
    build.go                     // New(providersDir, credentialsPath, stdout): the one public constructor that wires up this service's whole internal/ dependency graph — public because cmd/core can't import services/core/internal/... directly (Go's internal-import rule); rebuilds the Fetcher/Stager/Sinker/Scrape/Crawl/Executor graph fresh per job, parameterized by that job's own resolved spec
    /internal
      /job
        parser.go                 // job-parser: load-time validation of a spec (§8) before anything runs
        executor.go                // job-executor: run loop — walks the source (urls or crawl frontier), enforces limits, dispatches to a worker
        /workers                   // the different things job-executor can dispatch a unit of work to
          scrape.go                 // scrape (Core worker): base per-URL pipeline — fetch -> stage -> sink one item
          links.go                  // the real workers.LinkExtractor: walks a fetched HTML body's <a href> elements via goquery (already a dependency), resolving each relative href against the page's own FinalURL; non-http(s)/malformed hrefs are silently dropped, never counted anywhere
          /crawl
            crawl.go                 // crawl (Core worker): scrape + frontier logic (depth/maxPages/maxDuration/scope/include/exclude/sitemap/robots)
            dedupe.go                // claims each seed/sitemap-URL/discovered-link the moment it's found (before it's added to the frontier), so the same URL (fragment dropped, query params canonicalized) is never fetched or queued twice this crawl — always on, not a job spec field (JOB_SPEC_REFERENCE.md §3.2)
            robots.go                // the real crawl.RobotsChecker: fetches + caches <origin>/robots.txt (once per origin per crawl run) via a plain direct HTTP client — robots.txt/sitemap.xml are meant to be openly fetchable by any crawler, so neither routes through the job's configured fetch tier/proxy/provider. Parsing/matching via github.com/temoto/robotstxt, always against the wildcard `User-agent: *` group. A 4xx robots.txt response means no restrictions (matches Google's documented convention); any other failure (network error, other status, unparsable body) fails closed — every URL under that origin is treated as disallowed for the rest of the crawl
            sitemap.go               // the real crawl.SitemapFetcher: fetches <origin>/sitemap.xml the same way (plain direct client), parses via stdlib encoding/xml, follows one level of <sitemapindex> recursion into child sitemaps; no gzip support
      /fetch
        fetch.go                   // fetcher: dispatches to whichever single tier fetch.use selects; enforces limits.concurrency; on failure, retries that SAME tier (never a different one) up to limits.retries, per backoff.go's backoff timing
        throttle.go                 // enforces limits.rps; crawl's own frontier scheduling paces through this too, so a crawl and the scrape it drives share one rate limit
        backoff.go                  // backoff timing only: the exponential delay before retry attempt N. Cross-tier escalation (local -> proxy -> provider) is NOT a Core/job-spec feature in v1 — it's a CLI-level interactive flow (a job fails, the CLI suggests a different tier, and on confirmation generates + reruns a new job spec with a different fetch.use). A job-spec field to auto-escalate through the ladder without a CLI prompt is a possible later feature, deferred like __rpc/plugin; if it lands, it gets its own file then rather than pre-claiming this one's name.
        /tiers                      // the different tiers fetcher can dispatch to
          local.go                   // fetch tier: direct HTTP fetch with a TLS fingerprint profile (chrome/firefox/safari)
          proxy.go                   // proxy tier: fetch routed through a user-supplied proxy URL
          provider.go                // provider tier: fetch via a named manifest — auth (via pkg/credentials), request templating (via pkg/provider), response JSONPath extraction, error-kind mapping (via pkg/fault)
      /stage
        stager.go                  // stager: runs the `stages` pipeline over a fetched item, in declared order, dispatching each configured stage to the Extractor its `use` selects and merging their results into one `data` map — never has to arbitrate a key collision between two stages, since `pkg/jobspec` rejects any spec where that's possible before this ever runs
        /extractors                 // the different extraction methods a stage's `use` can select — this is the "Extractor" interface stager dispatches to (raw/markdown/css/plugin are extraction methods, not stages themselves; a "stage" is the configured {use, ...config} pipeline slot they get plugged into)
          raw.go                      // `raw` extractor -> data.raw: the untouched body
          markdown.go                 // `markdown` extractor -> data.markdown
          css.go                      // `css` extractor -> data[name] per selector(@attr)
          plugin.go                   // `plugin` extractor: thin adapter into pkg/plugin's pluginer; only legal under __rpc (JOB_SPEC_REFERENCE §6)
      /sink
        sinker.go                  // sinker: reads sink.use and dispatches a finished item to whichever sink implementation is selected
        /sinks                      // the different sink implementations sinker can dispatch to — this is the "sink" interface from the diagram
          stdout.go                   // stdout sink (default): streams NDJSON to stdout
          ndjson.go                   // ndjson sink: streams NDJSON to sink.path as items complete
          json.go                     // json sink: buffers every item, writes one JSON array to sink.path on completion
          stream.go                   // stream sink: used when a job is running over RPC — pushes items to the RPC streamer instead of a local file/stdout

  /cli                           // "CLI" container in the diagram
    service.go                    // Commander: parses argv, routes to a subcommand; maps a finished job's outcome to a process exit code via pkg/exitcode
    build.go                     // New(providersDir, coreService, credentialsPath, prompt): the one public constructor that wires up this service's whole internal/ dependency graph, same internal-import-rule reason as services/core/build.go
    /internal
      /command
        version.go                  // `pobear version` — no deps, just prints the binary version
        scrape.go                  // `pobear scrape <url|file>` — flags -> jobgen
        crawl.go                   // `pobear crawl <url>` — everything scrape has, plus crawl-only flags -> jobgen
        run.go                     // `pobear run <job.yaml|->` — parses an already-complete spec directly, skips jobgen
        providers.go                // `pobear providers [name]` — reads providerator
        auth.go                     // `pobear auth <provider>` — calls providerator's provider-auth.go
        flags.go                   // shared spf13/cobra flag registration, provider-option resolution, and the print-spec/run/output-format-raw execution glue every subcommand above calls into
        // note: these are CLI *subcommands* (flag parsing only) — distinct from services/core/internal/job/workers' scrape.go/crawl.go, which are the actual fetch/stage/sink workers. Same names, different layer.
        // each subcommand builds its own standalone *cobra.Command (not a shared command tree) purely for its flag parsing, Args validation, and auto-generated --help; service.go's own top-level dispatch (bare invocation, unknown-command, --help listing) is unchanged and doesn't go through cobra at all. A missing required positional argument prints that command's full help (flags.go's requireExactlyOne), and `--provider <name> --help` appends that provider's live-loaded option schema (flags.go's wireProviderHelp) — see CLI_REFERENCE.md.
      /jobgen
        jobgen.go                  // job-generator (CLI-side): flags (or wizard form state) -> a validated job spec; what --print-spec prints
      /providerator
        providerator.go            // providerator: loads/validates installed manifests, answers "what fields does provider X accept" for providers/auth/wizard/--opt validation
        provider-auth.go            // validates the provider name via providerator, then reads/writes its credential through pkg/credentials — what `pobear auth <provider>` and the wizard's inline "no credential" prompt call into
      /wizard
        tui.go                     // terminal-ui: `pobear wizard` (or bare `pobear`) full-screen reactive form; calls providerator for live field swapping, hands result to jobgen

  /rpc                           // "RPC" container in the diagram — now drawn nested inside a larger "SDK" frame together with SDK bindings, with an explicit SDK bindings -> RPC arrow; still lives in the `pobear` binary (services/rpc), not in /sdk's per-language packages, since it's the hidden `pobear __rpc` subcommand SDKs spawn; a distinct adapter pair from the CLI's, not a re-use of it
    service.go                    // communicator: speaks the __rpc wire protocol (JSON over stdin/stdout) to whatever spawned this subprocess; implements pkg/plugin's caller interface
    /internal
      /jobgen
        jobgen.go                  // job-generator (RPC-side): wire-protocol params -> a validated job spec — separate implementation from cli/internal/jobgen, same concept
      /streamer
        streamer.go                // streamer: the reverse direction — streams results back over the wire, and carries `plugin`-stage callbacks (only usable in __rpc mode, per JOB_SPEC_REFERENCE §6)

/providers                       // installed provider manifests (YAML), default $POBEAR_PROVIDERS_DIR — see PROVIDER_REFERENCE.md §13

/sdk                              // "SDK bindings" in the diagram — per-language client libraries, calling into services/rpc's communicator
  /python                          // e.g. spawns `pobear __rpc` and talks to it over the wire protocol
  /js                               // same pattern as /python, different language — see "Open items" below
```

### Open items

Things the diagram implies but the reference specs don't pin down yet:

- The exact message shape of the `__rpc` wire protocol (what `services/rpc`'s
  communicator/jobgen/streamer actually send and receive) isn't specified in any
  reference doc yet.
- Whether `/sdk` ships more than one language at launch, or starts as a single
  reference implementation the manifest format is designed to allow others to copy.
- `cli/internal/wizard`'s full-screen terminal UI (`CLI_REFERENCE.md`'s
  wizard section) is unbuilt and its own design session hasn't happened yet.
  Working assumption for that session: bring in a TUI library (tview or
  bubbletea) for the same reason `cli/internal/command` itself moved onto
  `spf13/cobra` for flag parsing/help/usage — a full-screen reactive TUI is
  exactly the kind of undifferentiated complexity a dedicated library exists
  to solve, more so than a flag parser.
