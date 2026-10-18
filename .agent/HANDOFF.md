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
- Charts draw with y and x axes and a titled border everywhere. Seven styles
  are selectable; braille is the default (braille, graph, solid, area, box,
  quadrant, ascii). Solid/area/graph fill the solar area; braille keeps clear
  dots. Tall charts get five y ticks and a dot grid, and smoothing defaults to
  3 samples. `v` switches to a source-mix stacked area, History year is a
  calendar heatmap, and Battery has a SOC arc. The Live chart
  plots solar, total consumption, grid and battery over a fixed 00:00-23:59
  window by default; `x` (or `display.day_window = "auto"`) switches to
  fit-to-samples. History gained a day mode.
- Day chart rows are sanity-checked: a kW-labelled power value of 100 or more
  is treated as watts, which keeps the day curve aligned with the detail
  endpoint on residential hardware.
- No credentials are available on this machine, so live field names, units and
  signs are still unverified.

## Known issues

- Some screens have thin empty/error states.
- Live field, unit and sign verification has not run; no credentials here.

## Verification

- `make check`: pass
- `go test ./...`: pass
- Golden frames at 60×20, 80×24 and 100×30, plus Live in auto window mode
- Live API: not run
