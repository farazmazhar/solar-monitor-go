# Testing

## Commands

```sh
make test    # go test ./...
make check   # gofmt, go vet, go test
make race    # go test -race ./...
```

Tests must not touch the network, real credentials or real databases. The
SolisCloud client is tested against `net/http/httptest`; everything else uses
the demo provider or fixtures.

## Test matrix

| Area | What is covered |
|---|---|
| Signing | golden vectors for `stringToSign`, `Content-MD5` and `Authorization`; compact JSON body |
| Client | success, non-zero code, HTTP 408 clock skew, malformed JSON, timeout, list pagination |
| Normalisation | kW→W and MWh→kWh conversion, missing `*Str`, alias fallbacks, string numbers, absent values staying nil |
| Signs | grid import/export mapping, battery charge/discharge mapping, config overrides |
| Flow solver | balance check, priority order, `sscCurrentFlowMap` preference |
| Config | defaults, TOML round-trip, legacy YAML import, env and flag precedence, invalid values |
| Store | migrations from empty, sample insert/query, aggregate upsert, pruning, WAL behaviour |
| Poller | backoff schedule, single in-flight request, staleness computation, manual refresh |
| UI | golden renders at 60×20, 72×20, 80×24, 100×30, 120×40 for every screen, both empty and populated |
| Charts | braille pixel mapping, axis ticks, cursor selection, sparkline scaling, bar stacking |
| Export | JSON and CSV shape, empty ranges, file naming |

## Golden views

Golden tests render the model with a fixed demo snapshot and a fixed terminal
size, then strip ANSI and compare against files in `internal/ui/testdata/`.
Update them deliberately with `go test ./internal/ui -update`, never blindly.

Sizes to cover: `60x20`, `72x20`, `80x24`, `100x30`, `120x40`. Each screen and
the help overlay should have at least one golden at 80×24; the responsive
breakpoints need one each for Live and Today.

## Demo provider

The demo provider is deterministic for a given simulated time, so it is safe
for golden tests. It advances a synthetic clock, so a repeated snapshot at the
same instant is stable; the UI tests pin the clock and stop the motion tick.

## Fixtures

- `internal/provider/solis/testdata/inverter_detail.json` — a redacted real
  payload once available. Until then, a hand-written fixture based on the
  vendor document.
- `internal/provider/solis/testdata/inverter_day.json`,
  `inverter_month.json`, `inverter_year.json`, `alarm_list.json`.
- The old project's Python implementation can be used to generate signing
  vectors locally; do not commit account identifiers.

## Manual checks

- `make demo` on an 80×24 terminal and on a wide terminal.
- `NO_COLOR=1 make demo` for the monochrome fallback.
- `make demo` with the terminal resized while the app runs.
- `make discover` once credentials exist; confirm fields, units and signs
  against `docs/API.md` and update aliases if needed.
