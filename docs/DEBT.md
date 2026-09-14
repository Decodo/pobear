# pobear known debt

Things we know are wrong, missing, or incomplete, that we've deliberately chosen
not to fix at the time they were found. This is not a backlog of features — it's
a record of specific gaps discovered mid-work, so they don't get silently
forgotten or rediscovered from scratch later. Each entry says what's wrong, why
it wasn't fixed on the spot, and what fixing it actually requires.

An entry is removed once it's fixed — this file tracks open debt, not a history
of resolved debt (git history already has that).

---

## `pobear wizard` (bare `pobear`) is entirely unimplemented

**What's wrong:** `CLI_REFERENCE.md`'s wizard section describes a full-screen
reactive terminal UI (`services/cli/internal/wizard/tui.go`, per
`PROJECT_STRUCTURE.md`), but `services/cli/internal/wizard` currently holds only
a `doc.go` — no types, no tests, no TUI dependency in `go.mod`. `services/cli.New`
doesn't register a `"wizard"` command at all. Bare `pobear` now prints a
clearer "interactive mode is not yet available" message (rather than the
confusing `unknown command "wizard"` it used to print), but there's still no
actual wizard to launch; `pobear wizard` explicitly still prints
`pobear: unknown command "wizard"` (exit 3), since nothing registers that name.

**Why not fixed now:** found while implementing the rest of `services/cli`
(providerator/jobgen/command) this session — every other package already had
real stub files and a full test-first suite to implement against; wizard has
neither, and is a categorically bigger, differently-shaped task (a reactive TUI,
needing its own design/brainstorm pass and a TUI library choice like
`bubbletea`) rather than "implement this stub to green."

**What fixing it requires:** a brainstorm/design pass picking a TUI approach,
writing the test-first spec for the form-reactivity rules `CLI_REFERENCE.md`
describes (field visibility changing with `Fetch via`/`Extract`/`Template`,
inline `Test`/`Save`/`Run` actions), adding the chosen TUI dependency, then
implementing `wizard.go`/`tui.go` against that spec, plus registering
`"wizard"` in `services/cli/build.go`'s command map.

**Blocks:** any real interactive `pobear`/`pobear wizard` invocation — no
fixture currently exercises it either, for the same reason.
