# pobear

## Documentation

`/docs` holds the authoritative specs for this project — read them before guessing:

- `docs/CLI_REFERENCE.md` — every `pobear` command, its flags, and exact output/exit-code behavior
- `docs/JOB_SPEC_REFERENCE.md` — the job spec document every front end produces and the engine executes
- `docs/PROVIDER_REFERENCE.md` — the provider manifest format (how a third-party scraping API plugs in)
- `docs/PROJECT_STRUCTURE.md` — the architecture and file/package layout, and how it maps to the design diagram
- `docs/DEBT.md` — known gaps deliberately left unfixed, why, and what fixing each one requires
- `docs/RELEASING.md` — how a release is cut and published, and where the `pobear version` string comes from

Rules:

- If a question comes up about CLI behavior, the job spec shape, the provider manifest format, or where something belongs architecturally, check the relevant doc in `/docs` first, rather than assuming or re-deriving it from scratch.
- If a genuine spec ambiguity blocks the work and there's no clear answer in `/docs`, ask rather than guess.
- When a real design decision gets made in conversation (a new field, a renamed component, a changed rule), update the corresponding doc in `/docs` in the same change — don't let the decision live only in code comments or chat history. If it doesn't fit an existing doc, add a new one under `/docs` and note it here.
- These docs are specs, not a changelog — keep them as the current authoritative shape of the thing, not a history of how it got there.
- When work surfaces a real gap that's deliberately left unfixed (out of scope for the current change, blocked on something else), record it in `docs/DEBT.md` — what's wrong, why it wasn't fixed now, what fixing it requires — rather than letting it live only in a plan doc or chat history. Remove the entry once it's actually fixed.
