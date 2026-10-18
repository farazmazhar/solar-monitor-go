# Handoff

Updated: 2026-10-18 20:55
Milestone: M1 complete; M2 mostly complete
Stopped at: working TUI with demo and live paths; tests pass

## Next command

```sh
cd /mnt/hdd/Compiler/git/solis-monitor-tui && make check
```

Then verify live fields when credentials exist:

```sh
make discover        # needs credentials in ~/.config/solar-monitor/config.toml
```

## State

- `go build ./...` and `make check` pass; `go test -race ./...` passes.
- Demo mode works with all seven screens; `--dump` renders any tab at any size.
- The SolisCloud client, config import, SQLite store and monitor service are
  implemented and unit tested.
- The project and binary were renamed from `solar-moonitor-go` to
  `solar-monitor-go` / `solar-monitor`.
- No credentials are available on this machine, so live field names, units and
  signs are still unverified.

## Known issues

- Some screens have thin empty/error states.
- Live field, unit and sign verification has not run; no credentials here.

## Verification

- `make check`: pass
- `go test ./...`: pass
- Manual `--dump` review at 60×20, 80×24, 100×30: looks correct
- Live API: not run
