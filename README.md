# solar-monitor-go

A read-only terminal UI for monitoring a Solis solar inverter through the
SolisCloud REST API. It talks to the cloud directly, keeps its own local
history, and ships a demo mode so the interface works without credentials.

```
 solar-monitor-go  Home Rooftop · S6-EH1P6K-L-PRO         ● ONLINE   just now
  Live  Today  History  Battery  PV  Alarms  Diagnostics
              ╭─ PV ─────────────────╮
              │ 5.67 kW    2 strings │
              ╰──────────────────────╯
                          ▼
  ╭─ GRID ─────────────╮   ╭─ HOUSE ────────────╮   ╭─ BATTERY ──────────╮
  │ 3.22 kW     EXPORT │◄──│ 656 W    bkp 256 W │──►│ 62%    1.80 kW CHG │
  ╰────────────────────╯   ╰────────────────────╯   ╰────────────────────╯
```

## Status

Early but working. The seven screens render from real or simulated data, the
SolisCloud client is implemented and tested against a fake server, and local
history is stored in SQLite. Live field and sign verification against a real
account is still pending.

## Requirements

- Go 1.27 or newer to build from source.
- SolisCloud API credentials for live data: API key, API secret and station ID
  from the SolisCloud dashboard (`Personal → API Management`).
- No C toolchain: the SQLite driver is pure Go.

## Build and run

```sh
make build      # ./solar-monitor
make run        # the TUI from source
make demo       # simulated data, no credentials, no network
make check      # gofmt, vet and the test suite
```

Direct equivalents:

```sh
go build -o solar-monitor ./cmd/solar-monitor
./solar-monitor --demo
```

Useful flags: `--config PATH`, `--state-dir PATH`, `--theme solar-dark|transparent`,
`--chart-style solid|area|box|quadrant|braille|ascii|graph`, `--no-motion`, `--init`, `--discover`,
`--version`.

`--dump` renders one frame with demo data and exits; `--dump-tab 0..6`,
`--dump-history day|month|year`, `--dump-window day|auto`,
`--dump-view power|mix` and `--dump-size 100x30` choose what to render. It is meant for screenshots,
debugging and CI smoke checks.

## Configuration

The config lives at `~/.config/solar-monitor/config.toml`. `solar-monitor
--init` writes a template; on first run the app also imports credentials from
the older `~/.config/solis2kde/config.yaml` if it exists.

```toml
[solis]
base_url = "https://www.soliscloud.com:13333"
api_key = "YOUR_API_KEY"
api_secret = "YOUR_API_SECRET"
station_id = "YOUR_STATION_ID"
timeout = "30s"

[poll]
interval = "5m"        # never below 1m
alarms = "15m"
history = "15m"

[display]
theme = "solar-dark"
motion = true
chart_style = "solid"  # solid | area | box | quadrant | braille | ascii | graph
currency = "PKR"
currency_symbol = "₨"
co2_kg_per_kwh = 0.4
# "day" pins the charts to 00:00-23:59 and lets the lines progress.
# "auto" fits the measured samples to the chart width instead.
day_window = "day"

[tariff]
import = 0.0
export = 0.0

[storage]
path = "~/.local/state/solar-monitor/monitor.db"
raw_snapshot_days = 30
```

Environment overrides: `monitor_API_KEY`, `monitor_API_SECRET`,
`monitor_STATION_ID`, `monitor_INVERTER_ID`, `monitor_CONFIG`,
`monitor_STATE_DIR`, `monitor_THEME`.

The database keeps five-minute samples forever (roughly 10 MB per year) and
prunes raw JSON snapshots after 30 days by default.

## Keys

| Key | Action |
|---|---|
| `1`–`7` | switch screen |
| `tab` / `shift+tab` | next / previous screen, or cycle a filter |
| `←` `→` | move the chart cursor |
| `j` `k`, `↑` `↓` | move in tables and scroll raw data |
| `r` | refresh now |
| `m` | toggle motion |
| `x` | toggle the chart window between fixed day and fit samples |
| `v` | toggle the day chart between power lines and the source-mix stack |
| `t` | jump to today or the current period |
| `s` `l` `g` `b` | toggle the Today chart series |
| `h` `l` | previous / next period in History |
| `e` | export JSON or CSV |
| `?` | help |
| `q` | quit |

Mouse clicks switch tabs and tiles, move the chart cursor, and scroll lists.

## Screens

- **Live** — energy-flow diagram, live tiles with sparklines, a battery gauge
  and a day chart with time and power axes. The chart plots solar, total
  consumption, grid and battery with an area fill under solar; `x` switches
  between a fixed 00:00–23:59 window and a fit-to-samples window. `v` switches
  to a stacked source-mix chart (solar / battery / grid feeding consumption).
- **Today** — the same day chart with a summary panel, a cursor and per-series
  toggles.
- **History** — day, month and year views. Day shows the same power chart for
  the selected date; month shows a bar chart; year shows a calendar heatmap of
  daily generation. `tab` cycles the mode and `h`/`l` step periods.
- **Battery** — SOC gauge, SOH, power, voltage, current, throughput and a 24 h
  SOC chart.
- **PV** — per-string voltage, current, power and share.
- **Alarms** — active, recent and all alarms with advice.
- **Diagnostics** — connection, last error, store statistics, raw snapshot
  browser and exports.

## Privacy and safety

The app is strictly read-only: it never sends control commands to the inverter.
Credentials stay in the mode-0600 config file, and captured payloads are stored
only in the local state directory.

## Development

```sh
make test    # go test ./...
make race    # go test -race ./...
make check   # formatting, vet and tests
```

Golden frames live in `internal/ui/testdata`; regenerate them deliberately with
`go test ./internal/ui -update`. See `AGENTS.md` and `docs/` for architecture,
API notes and the current status.

## License

MIT. See `LICENSE`.
