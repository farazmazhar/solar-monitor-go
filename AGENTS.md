# AGENTS.md — solar-moonitor-go

This file is the operating manual for any agent or human working in this
repository. Read it first, every session. It is stable: it does not hold
progress. Progress lives in `docs/STATUS.md` and `.agent/HANDOFF.md`.

## Mission

A good-looking, read-only terminal UI for monitoring a Solis solar inverter
through the SolisCloud REST API. It talks to the cloud directly and keeps its
own local history. It never writes to the inverter, never controls anything,
and never modifies the older `solis-kde-plasma-sensors` project next door.

The provider layer is meant to grow into other solar APIs later. Only Solis is
in scope now; do not build speculative providers.

## Start a session

1. Read `.agent/HANDOFF.md` — where the last session stopped.
2. Read `docs/STATUS.md` — what is done, in progress, next, blocked.
3. Skim the newest session note in `.agent/sessions/`.
4. Run `make check` (or `go test ./...` if the Makefile is not usable yet).
5. Take the first unchecked item under `Next` in `docs/STATUS.md` unless the
   user asked for something else.

`HANDOFF.md` always ends with the exact next command and next task, so a
session with no other context can resume deterministically.

## End a session

1. Update `docs/STATUS.md`: move finished work to `Done`, state what is in
   progress, restate what is next, note anything blocked and why.
2. Append a dated entry to `.agent/sessions/YYYY-MM-DD.md`: what changed, why,
   what was verified, what failed, and the next command.
3. Rewrite `.agent/HANDOFF.md` to the current state. It is a snapshot, not a
   log; keep it short.
4. Record any decision that changes architecture, scope, or user-visible
   behaviour in `docs/DECISIONS.md`.
5. Run the tests and report the result honestly, including failures.
6. Commit when the tree is coherent. Milestone-sized commits are fine.

Documentation is part of "done", not a follow-up task.

## Invariants

- **Read-only.** No inverter control, no device writes, no settings endpoints.
- **No secrets in the repo.** Never commit API keys, `config.toml` with real
  credentials, real state databases, or captured payloads with account data.
- **Respect the cloud.** Detail polling defaults to 300 s and must not go below
  60 s. The API allows 2 requests/second; stay well under it.
- **Never invent data.** Missing values render as `—`, stale data carries an
  age badge. Nothing is silently coerced to zero.
- **One unit convention.** Store watts and kilowatt-hours internally; convert
  only for display, using the theme/format helpers.
- **Demo is sandboxed.** Demo mode uses an in-memory database and never touches
  real history or the network.
- **Colours come from theme tokens.** No raw colour literals in view code.
- **The old project is reference only.** Do not edit
  `../solis-kde-plasma-sensors`; read it for API/signing background.

## Commands

```sh
make build      # build ./solar-moonitor
make run        # run the TUI from source
make demo       # run the TUI with simulated data (no credentials)
make test       # go test ./...
make check      # gofmt check, go vet, go test
make discover   # print a live inverterDetail dump (needs credentials)
```

Direct equivalents, when the Makefile is unavailable:

```sh
go build -o solar-moonitor ./cmd/solar-moonitor
go run ./cmd/solar-moonitor --demo
go run ./cmd/solar-moonitor --discover
go test ./...
```

## Layout

```
cmd/solar-moonitor/          entry point, flags, wiring
internal/config/             TOML load, defaults, legacy import, env/flags
internal/provider/           provider-agnostic domain types and Source interface
internal/provider/solis/     SolisCloud client, signing, field normalisation
internal/provider/demo/      deterministic simulated provider
internal/store/              SQLite schema, migrations, samples, aggregates
internal/monitor/            polling, backoff, staleness, state assembly
internal/ui/                 root Bubble Tea model, theme, tabs, hit testing
internal/ui/screens/         one file per screen
internal/ui/components/      flow diagram, tiles, gauge, charts, tables
docs/                        living documentation (see the map below)
.agent/                      handoff and session notes for agents
```

## Documentation map

| Question | Read |
|---|---|
| What is the plan? | `docs/PLAN.md` |
| What is the state right now? | `docs/STATUS.md`, `.agent/HANDOFF.md` |
| How does the SolisCloud API work? | `docs/API.md` |
| Why was something built this way? | `docs/DECISIONS.md` |
| What should the UI look like? | `docs/UI.md` |
| How do I test it? | `docs/TESTING.md` |
| Background research | `docs/RESEARCH.md` |

## Conventions

- Go 1.27, standard formatting (`gofmt`), `go vet` clean.
- Wrap errors with context: `fmt.Errorf("solis: fetch detail: %w", err)`.
- No global mutable state outside `main`. Pass dependencies explicitly.
- Table-driven tests; use `httptest` for the API and golden strings for views.
- Keep Bubble Tea `Update` pure and fast; put I/O in commands.
- UI code never imports provider-specific packages; it consumes
  `internal/provider` types only.
- Prefer small files with one responsibility over large multi-purpose files.
