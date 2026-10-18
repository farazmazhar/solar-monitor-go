# UI reference

## Look

A painted dark canvas with a restrained solar palette, rounded panels, and
generous spacing between sections. Colour is never the only signal: direction,
labels, glyphs and a legend carry meaning too.

## Palette

| Token | Hex | Used for |
|---|---|---|
| Background | `#0B0F14` | painted canvas |
| Text | `#E6EDF3` | primary values |
| Dim | `#8B98A5` | labels, secondary values |
| Faint | `#22303C` | rules, separators, borders |
| Solar | `#F5A524` | PV generation |
| Green | `#22C55E` | generation, good state |
| Grid | `#4C8DF6` | grid import/export |
| Export | `#22D3EE` | exporting |
| Import | `#F97316` | importing |
| Battery | `#A78BFA` | SOC, charge |
| Discharge | `#F472B6` | discharge |
| Danger | `#EF4444` | faults, low SOC, errors |
| Accent | `#F5A524` | active tab, focus, chart titles |

Themes: `solar-dark` (default) and `transparent` (no painted background).
`NO_COLOR` falls back to a monochrome rendering built from bold, faint and
reverse video. 256-colour terminals degrade through Lip Gloss.

## Chrome

```
 solar-monitor-go  Home Rooftop · S6-EH1P6K-L-PRO        DEMO   simulated  14:32
  Live  Today  History  Battery  PV  Alarms  Diagnostics               Sun 18 Oct
```

- Header: wordmark, station, model, state pill, sample age, clock, alarm count.
- Tab bar: active tab filled with the accent, inactive dim. Digits `1`–`7`
  switch directly.
- Footer: contextual keys on the left, next-refresh countdown and last error on
  the right.
- Help overlay on `?`; `esc` closes.

## Panels and spacing

Sections are separated by a blank line, and anything that reads as a chart or a
table is framed with a rounded border carrying its title in the top edge
(`╭─ TITLE ────╮`). Content inside a panel is indented by two spaces. The rule
is: one idea per panel, one blank line between panels.

## Charts

All charts are custom renderers in `internal/ui/chart.go`.

- **Line charts** can be drawn in one of seven styles, set with
  `display.chart_style`:
  - `braille` (default) — the 2×4 dot matrix, smooth but sparse.
  - `solid` — half-block glyphs (`▀ ▄ █`), two vertical sub-rows per
    cell, thick continuous strokes, with an area fill under solar.
  - `area` — as `solid` but with a denser (`▒`) generation fill.
  - `box` — box-drawing glyphs (`─ │ ╱ ╲ ┼`), thin lines with real junctions.
  - `quadrant` — quadrant blocks (`▖ ▗ ▘ ▝ ▚ ▞ █`) at 2×2 sub-cell resolution.
  - `graph` — ntcharts' canvas line renderer, thin lines with rounded corners.
  - `ascii` — `- | / \ + *`, for minimal fonts; also the `NO_COLOR` fallback.
  All styles draw a y axis, an x axis, a dashed zero line when the range crosses
  zero, and accept a cursor and any number of evenly distributed x labels.
  Tall charts get five y ticks and a subtle dot grid at the tick rows and
  x-label columns. Where two braille series share a cell, their colours blend
  so an overlap is not mistaken for a colour change in one line. `display.chart_smoothing` (default 3) applies a
  centred moving average; 1 turns it off, 5 softens cloud spikes further.
- **Day charts** share labels of `00:00`, `06:00`, `12:00`, `18:00` and `23:55`.
- **Bar charts** use eighth-block columns with a y axis and a labelled x axis,
  and are used for month and year history.
- **Sparklines** are single-row block charts used inside Live tiles.

### Fixed-day versus fit-samples

`x` toggles the Live chart window, and `display.day_window` sets the default:

- `day` (default) pins the x axis to the full day. The axis never moves; only
  the plotted lines grow as samples arrive.
- `auto` fits the measured samples to the width, so the visible window follows
  the data.

Today, History's day view and the Battery SOC chart are always fixed-day.

### Area fill and source mix

Under `solid`, `area` and `graph`, the solar line is filled down to the zero
baseline in a light shade, so the shape of the generation curve is visible even when the
chart is shallow. Braille leaves the area clear so the thin dots stay readable. Press `v` on Live, Today or History's day view to switch the
same chart between:

- **Power** — solar, consumption, grid and battery lines.
- **Source mix** — a stacked area of solar, battery discharge and grid import,
  bottom to top, with the stack totalling household consumption. This is the
  clearest view of where the house power came from.

### History year view

On wide terminals (110 columns and 28 rows or more) the History year view is a
calendar heatmap: one row per month, one column pair per day, shaded on a
dark-to-bright amber ramp by that day's generation. Each row shows its monthly
total, the legend prints the quartile values in kWh, and a monthly bar chart
sits below when there is height left. Smaller terminals get twelve monthly bars
instead, which read better and skip the daily requests entirely.

Heatmap data is cache-first: each month's daily rows are read from SQLite and
only missing or incomplete months are fetched, paced at roughly two requests per
second on the real API. A complete year loads from disk with no network calls.

### Battery arc

The Battery screen draws SOC as a semicircular arc with the percentage in the
middle, filled from left to right and coloured green/amber/red by level. The
24-hour SOC chart below stays fixed-day.

## Live screen

```
             ╭ PV ─────────────────╮
             │ 5.67 kW    2 strings │
             ╰──────────┬──────────╯
                        ▼
 ╭ GRID ─────────────╮  ╭ HOUSE ────────────╮  ╭ BATTERY ──────────╮
 │ 3.22 kW     EXPORT │◄─│ 656 W    bkp 256 W│─►│ 62%    1.80 kW CHG│
 ╰────────────────────╯  ╰──────────────────╯  ╰───────────────────╯
                    backup 256 W   ·   smart 401 W
```

Arrow direction and colour follow the live flow. Below the diagram: tiles
(solar, house, grid, battery and, when wide enough, income and CO₂), the
segmented battery gauge with a reserve tick, a colour legend, and the day chart
with time and power axes. The chart plots solar, total consumption, grid and
battery.

- The flow solver is described in `docs/API.md`.
- Tiles collapse from four rows to three or two as height shrinks.

## Screens

- **Today** — title with peak power and time, a three-line summary panel, the
  series legend with `s l g b` toggles, the cursor line, and the bordered day
  chart with axes.
- **History** — `tab` cycles day / month / year, `h` and `l` step periods, `t`
  returns to now. Day shows the same power chart for the selected date. Month
  and year show a summary panel and a bordered bar chart with a y axis and
  labelled columns.
- **Battery** — SOC gauge, SOH, power and direction, voltage, current,
  temperature, today and lifetime throughput, reserve and charge limits, fault
  codes, and a fixed-day SOC chart with a 0–100 % axis.
- **PV** — a summary line and a bordered strings table with voltage, current,
  power and a share bar.
- **Alarms** — `tab` cycles active / 30 days / all; a table with level, start,
  code, message and state, plus a detail pane with advice.
- **Diagnostics** — connection, latency, last error, config and database paths,
  store statistics, versions, terminal profile, raw snapshot browser, export
  and re-discover.

## Responsive rules

| Width | Behaviour |
|---|---|
| ≥ 100 | six tiles including income and CO₂; four tile rows; taller charts |
| 80–99 | four tiles; full flow diagram; day chart |
| 72–79 | two-by-two tiles; compact flow strip; shorter chart |
| < 72 | single column; flow strip; values stack; secondary detail hidden |

Height matters too: under 26 rows the tile sparkline row is dropped, and under
24 rows tiles collapse to label, value and secondary line only, which gives the
chart more room.

## Keys

| Key | Action |
|---|---|
| `1`–`7` | switch tab |
| `tab` / `shift+tab` | next / previous tab, or cycle a mode or filter |
| `h` / `l`, `←` / `→` | step periods or move the chart cursor |
| `j` / `k`, `↑` / `↓` | move in tables and scroll |
| `r` | refresh now |
| `m` | toggle motion |
| `x` | toggle the fixed-day / fit-samples chart window |
| `v` | toggle the power lines / source-mix stacked area |
| `t` | jump to today / current period |
| `s l g b` | toggle Today series |
| `e` | export JSON or CSV on History and Diagnostics |
| `d` | re-discover the inverter on Diagnostics |
| `?` | help overlay |
| `esc` | close overlay |
| `q` | quit |

Mouse: click tabs and tiles, click a chart to move the cursor, wheel to scroll
tables and the raw browser.

## Motion

A slow pulse on active flow arrows and a soft blink on the live dot. Only on
the Live screen, only when motion is enabled, and stopped when the terminal is
not focused. `m` toggles it; `--no-motion` disables it at startup.

## States

Every screen has explicit loading, empty and error states. Empty says what is
missing in plain words. Errors show the provider message and the age of the
last good snapshot. Stale data keeps rendering with an amber age badge rather
than disappearing.
