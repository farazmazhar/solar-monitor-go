# UI reference

## Look

A painted dark canvas with a restrained solar palette, rounded panels, and
clear labels. Colour is never the only signal: direction, labels and glyphs
carry meaning too.

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
| Accent | `#F5A524` | active tab, focus |

Themes: `solar-dark` (default) and `transparent` (no painted background).
`NO_COLOR` falls back to a monochrome rendering. 256-colour terminals use the
nearest palette entries.

## Chrome

```
 SOLIS MONITOR   Home Rooftop · S6-EH1P6K-L-PRO       ● ONLINE  14:32  2m ago
 1 Live   2 Today   3 History   4 Battery   5 PV   6 Alarms   7 Diagnostics
```

- Header: wordmark, station, model, online pill, clock, sample age, active
  alarm count.
- Tab bar: active tab filled with the accent, inactive dim; numbers are the
  direct keys.
- Footer: contextual keys on the left, next-refresh countdown and last error on
  the right.
- Help overlay on `?`; `esc` closes.

## Live screen

```
              ╭ PV  3.42 kW ╮
              │ 2 strings   │
              ╰──────┬──────╯
                     ▼
 ╭ GRID ───────╮  ╭─────────────╮  ╭ BATTERY ─────╮
 │ 0.80 kW EXP │◄─│   HOUSE     │─►│ 78% · 0.72CHG│
 │ 230V · 50Hz │  │   1.90 kW   │  │ 51.2V · 14A  │
 ╰─────────────╯  ╰──────┬──────╯  ╰──────────────╯
                    ┌────┴────┐
                    ▼         ▼
              BACKUP 0.30  SMART 1.60
```

Arrow direction and colour follow the live flow. At night, PV dims to the faint
token and shows `0 W`. Below the diagram: four tiles (Solar, House, Grid,
Battery) with a value, a secondary line, and a sparkline; a segmented SOC gauge
with a reserve tick; and a mini today curve.

- Space is the node layout, not a full Sankey.
- The flow solver is described in `docs/API.md`.
- Widgets at 80 columns are exactly 19 cells wide with 2-cell gaps.

## Screens

- **Today** — multi-series braille chart, series toggles `s/l/g/b`, cursor with
  time and per-series values, summary strip with generated, consumed, imported,
  exported, peak power and peak time.
- **History** — `tab` toggles month/year; `h/l` step periods; `t` returns to the
  current period. Month bars show generation with import/export/battery context;
  year bars show monthly totals. Summaries include PKR income and CO₂ avoided.
- **Battery** — large segmented gauge, SOC/SOH, power and direction, voltage,
  current, temperature, today and lifetime throughput, reserve and charge
  limits, fault codes, 24 h SOC chart.
- **PV** — table of active strings with V/A/W and a share bar; MPPT totals; DC
  input count; inverter temperature; DC/AC efficiency when both sides exist.
- **Alarms** — `tab` cycles active / 30 days / all; table with level, start,
  code, message and state; detail pane with advice; empty state when clear.
- **Diagnostics** — connection, latency, last code/message, config and DB paths,
  sample counts, versions, terminal profile, raw snapshot browser, export
  actions, re-discover.

## Responsive rules

| Width | Behaviour |
|---|---|
| ≥ 100 | six tiles including income and CO₂; taller charts; fuller battery detail |
| 80–99 | four tiles; flow diagram; chart shown |
| 72–79 | two-by-two tiles; compact flow strip; shorter chart |
| < 72 | single column; flow strip; values stack; secondary detail hidden |

Width below 72 columns shows a message rather than breaking the layout.

## Keys

| Key | Action |
|---|---|
| `1`–`7` | switch tab |
| `tab` / `shift+tab` | next / previous tab, or cycle filter inside a screen |
| `h` / `l`, `←` / `→` | move selection or chart cursor |
| `j` / `k`, `↑` / `↓` | move in tables and scroll |
| `r` | refresh now |
| `m` | toggle motion |
| `t` | jump to today / current period |
| `s l g b` | toggle Today series |
| `e` | export JSON or CSV on Diagnostics/History |
| `?` | help overlay |
| `esc` | close overlay |
| `q` | quit |

Mouse: click tabs, click a tile to open its related screen (Solar→PV,
Battery→Battery, Grid→Today, House→Today), click the chart to move the cursor,
wheel to scroll tables and the raw browser.

## Motion

A slow pulse on active flow arrows and a soft blink on the live dot. Only on
the Live screen, only when motion is enabled, and stopped when the terminal is
not focused. `m` toggles it; `--no-motion` disables it at startup.

## States

Every screen has explicit loading, empty and error states. Loading shows the
canvas with a spinner in the header. Empty says what is missing in plain words.
Errors show the provider message and the age of the last good snapshot. Stale
data keeps rendering with an amber age badge rather than disappearing.
