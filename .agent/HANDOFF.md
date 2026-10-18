# Handoff

Updated: 2026-10-18 20:35
Milestone: M1 mostly complete; M2 partial
Stopped at: working TUI with demo and live paths; tests pass

## Next command

```sh
cd /mnt/hdd/Compiler/git/solis-monitor-tui && make check
```

Then either start the poller-backoff task, or verify live fields:

```sh
make discover        # needs credentials in ~/.config/solar-moonitor/config.toml
```

## State

- `go build ./...` and `make check` pass.
- Demo mode works with all seven screens; `--dump` renders any tab at any size.
- The SolisCloud client, config import, SQLite store and monitor service are
  implemented and unit tested.
- No credentials are available on this machine, so live field names, units and
  signs are still unverified.

## Known issues

- The poller retries on the normal interval; exponential backoff is not
  implemented yet.
- `NO_COLOR` and 256-colour fallback are not implemented; the theme falls back
  to solar-dark for unknown names and transparent for `theme = "transparent"`.
- Some screens have thin empty/error states.
- The name `solar-moonitor-go` was never explicitly re-confirmed after the user
  wrote it. If it was a typo, rename module, binary, docs and paths together.

## Verification

- `make check`: pass
- `go test ./...`: pass
- Manual `--dump` review at 60×20, 80×24, 100×30: looks correct
- Live API: not run
