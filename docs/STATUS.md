# Status

Updated: 2026-10-18
Current milestone: M1 complete; M2 mostly complete
Overall state: the TUI runs end to end on demo data and on the real API path,
with local SQLite history. Tests pass. Live field verification is the main gap.

## Next

- [ ] Live verification: run `make discover` with real credentials (or a saved
      `inverterDetail` dump) and confirm field aliases, units and signs against
      `docs/API.md`, then fix any mismatch
- [ ] Empty/loading/error states on every screen (partial today)
- [ ] Embed golden screenshots in the README
- [ ] Tag a first version

## Blocked

- [ ] Live SolisCloud verification — no credentials on this machine. The app
      runs in demo mode; `--discover` is ready for when credentials exist.

## Done

- [x] Repository foundation, AGENTS.md, docs, Makefile, git history
- [x] Config loader with defaults, env overrides and legacy KDE import
- [x] Provider interface plus deterministic demo provider
- [x] SolisCloud client: signing, discovery, detail, day, month, year, alarms
- [x] Unit and sign normalisation with alias fallbacks
- [x] SQLite store: snapshots, samples, daily/monthly aggregates, alarms, stats
- [x] Monitor service: records snapshots, merges local and remote day data,
      falls back to cache when the cloud is down
- [x] Poller with exponential backoff (30 s → 5 m), single in-flight request
      and staleness tracking
- [x] Re-discover the inverter from the Diagnostics screen (`d`)
- [x] Bubble Tea UI with seven screens, flow diagram, charts and gauge
- [x] Chart pass: y and x axes everywhere, bordered/framed charts and panels,
      fixed-day 00:00-23:59 window with `x`/`day_window` toggle, History day
      mode, Today restructured with a summary panel and series legend
- [x] Seven chart styles (solid, area, box, quadrant, braille, ascii, graph)
- [x] Solar area fill; `v` source-mix stacked area; year calendar heatmap;
      battery SOC arc
- [x] Mouse navigation, help overlay, motion toggle, JSON/CSV export
- [x] `NO_COLOR` monochrome theme and a transparent theme
- [x] Golden render tests at 60×20, 80×24 and 100×30
- [x] Unit tests for signing, normalisation, config, demo, store and monitor
- [x] README and LICENSE

## Verification

- `make check`: passes (gofmt, go vet, go test ./...)
- `go test -race ./...`: passes
- `make demo`: runs; `--dump` renders every screen at 60×20, 80×24 and 100×30
- `NO_COLOR=1 ./solar-monitor --dump`: renders without colour
- Live API check: cannot run without credentials

## Notes

- App name is `solar-monitor-go`; binary is `solar-monitor`; config and state
  directories are `solar-monitor`.
- Currency is PKR (₨).
- Raw samples are retained forever; raw JSON snapshots prune after 30 days.
- Scope is strictly read-only.
- Run `go test ./internal/ui -update` only when a UI change is intended.
