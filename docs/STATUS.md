# Status

Updated: 2026-10-18
Current milestone: M1 mostly complete; M2 partial
Overall state: the TUI runs end to end on demo data and on the real API path.
Tests pass. Live field verification is the main gap.

## In progress

- [ ] Poller backoff: failures currently retry on the normal interval instead of
      backing off (30 s → 1 m → 2 m → 5 m)

## Next

- [ ] Live verification: run `make discover` with real credentials (or a saved
      `inverterDetail` dump) and confirm field aliases, units and signs against
      `docs/API.md`
- [ ] Re-discover inverter from the Diagnostics screen
- [ ] `NO_COLOR` and 256-colour fallback
- [ ] Empty/loading/error states on every screen
- [ ] Embed golden screenshots in the README
- [ ] Exponential backoff in the poller

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
- [x] Bubble Tea UI with seven screens, flow diagram, charts and gauge
- [x] Mouse navigation, help overlay, motion toggle, JSON/CSV export
- [x] Golden render tests at 60×20, 80×24 and 100×30
- [x] Unit tests for signing, normalisation, config, demo, store and monitor
- [x] README and LICENSE

## Verification

- `make check`: passes (gofmt, go vet, go test ./...)
- `make demo`: runs; `--dump` renders every screen at 60×20, 80×24 and 100×30
- Live API check: cannot run without credentials

## Notes

- App name is `solar-moonitor-go`; binary is `solar-moonitor`; config and state
  directories are `solar-moonitor`.
- Currency is PKR (₨).
- Raw samples are retained forever; raw JSON snapshots prune after 30 days.
- Scope is strictly read-only.
- Run `go test ./internal/ui -update` only when a UI change is intended.
