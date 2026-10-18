# Plan — solar-monitor-go

Status: agreed on 2026-10-18. Scope changes are recorded in
`docs/DECISIONS.md` and mirrored into `docs/STATUS.md`.

## Product

A read-only TUI for a Solis S6-EH1P6K-L-PRO inverter, fed by the SolisCloud
REST API and a local SQLite history. It runs standalone beside the existing
`solis-kde-plasma-sensors` daemon and never controls the inverter.

The provider layer is deliberately provider-agnostic so other solar APIs can be
added later. No second provider is in scope.

## Decisions in force

| Area | Decision |
|---|---|
| Name | repo/module `solar-monitor-go`, binary `solar-monitor`, display title `solar-monitor-go` |
| Stack | Go 1.27, Bubble Tea v2, Lip Gloss v2, Bubbles v2, `charmbracelet/x/ansi` |
| Charts | custom braille line chart, block bar chart, block sparkline (see ADR-0006) |
| Storage | SQLite via `modernc.org/sqlite`, pure Go, no cgo |
| Data source | direct SolisCloud API, read-only |
| Config | `~/.config/solar-monitor/config.toml`, TOML, auto-import from the old KDE config |
| State | `~/.local/state/solar-monitor/monitor.db` |
| Money | PKR (₨); API income when the currency matches, otherwise configured tariff |
| Visual | painted dark canvas, solar palette, boxed flow diagram |
| Cadence | 300 s default, configurable, never below 60 s |
| History | 5-minute samples kept forever; raw JSON snapshots pruned after 30 days |
| Demo | `--demo`, in-memory database, deterministic simulation |

## Screens

Seven tabs: Live, Today, History, Battery, PV, Alarms, Diagnostics.

- **Live** — boxed energy-flow diagram, live tiles with sparklines, a segmented
  SOC gauge, a colour legend, and a bordered day chart with time and power axes
  plotting solar, total consumption, grid and battery, with an area fill under
  solar. `x` switches the chart between a fixed 00:00–23:59 window and a
  fit-to-samples window; `v` switches to a stacked source-mix area.
- **Today** — summary panel, series legend with `s l g b` toggles, cursor line,
  and a bordered fixed-day chart with axes.
- **History** — day, month and year modes. Day shows the power chart for the
  selected date; month shows a bordered bar chart; year shows a calendar
  heatmap of daily generation. Summaries include income and CO₂.
- **Battery** — semicircular SOC arc, SOH, charge/discharge power, voltage and
  current, today and lifetime throughput, reserve limits, and a fixed-day SOC
  chart.
- **PV** — bordered strings table with V/A/W and share bars; MPPT totals; DC
  input count; inverter temperature; DC/AC efficiency where available.
- **Alarms** — active / recent 30 days / all filters, table with level, time,
  code, message and state, detail pane with advice.
- **Diagnostics** — connection and latency, last API code/message, config and
  DB paths, sample counts, versions, raw snapshot browser, JSON/CSV export.

Navigation is top tabs (keys 1–7 or tab/shift-tab), vim keys and arrows,
optional mouse for tabs, tiles and chart cursor. Motion is subtle and toggleable.

## Architecture

```
cmd/solar-monitor        flags, config resolution, program startup
internal/config           TOML, defaults, legacy import, env, flags
internal/provider         domain types, Source interface
internal/provider/solis   signed client, decoding, unit/sign normalisation
internal/provider/demo    deterministic simulation of the same interface
internal/store            SQLite schema, migrations, samples, aggregates
internal/monitor          poller, backoff, staleness, assembled state
internal/ui               root model, theme, tabs, keymap, hit registry
internal/ui/screens       one file per screen
internal/ui/components    flow, tiles, gauge, charts, tables, status bar
```

The UI depends on `provider.Source` only. Real and demo data take the same
path, so demo mode exercises the real rendering code.

## Normalisation rules

- Store W and kWh; convert to kW/MWh only for display.
- Values are nullable; absence is rendered as `—`.
- Parse `*Str` unit companions. Power is usually kW, energy usually kWh, but
  totals may be MWh.
- Field aliases, in priority order:

| Metric | Candidates |
|---|---|
| Solar | `pac` |
| Grid signed | `psumOrgin` → `psum` → `acGridPowerA` |
| House load | `familyLoadPowerOrigin` → `familyLoadPower` → `totalLoadPower` |
| Backup | `backupPowerA` → `bypassLoadPower` |
| Battery power | `batteryPower` → `batteryVoltage × bstteryCurrent` |
| Battery SOC | `batteryCapacitySoc` → `batteryPercent` |
| Strings | `powN`/`mpptPowN`, fallback `uPvN × iPvN` |
| Today energy | `eToday` |
| Lifetime energy | `allEnergyOriginal` → `eTotal` (unit-aware) |
| Grid energy | `gridPurchased*Energy`, `gridSell*Energy` |
| Battery energy | `battery*ChargeEnergy`, `battery*DischargeEnergy` |

Grid sign: positive = export, negative = import (verify live).
Battery sign: positive = charge, negative = discharge (verify live).
The flow diagram prefers `sscCurrentFlowMap`; otherwise a solver derives flows
and runs a balance check, surfacing a diagnostics warning on mismatch.

## Milestones

### M0 — foundation and visual proof

- [x] M0.0 repo, AGENTS.md, docs scaffold, Makefile, git history
- [x] M0.1 Go scaffold, config, provider interface, demo provider
- [x] M0.2 theme, tab shell, Live screen at 80×24, golden render tests
- [x] M0.3 Today, History, Battery, PV, Alarms, Diagnostics screens
- [x] M0.4 responsive 60×20 / 100×30, help overlay, motion, mouse, tests

### M1 — API and storage

- [x] SolisCloud client with HMAC-SHA1 signing and golden-vector tests
- [x] Legacy config import and `--init`
- [x] SQLite schema, migrations, sample and aggregate queries
- [x] Poller with exponential backoff, single in-flight request and staleness
- [x] `--discover` field dump
- [x] Fake-server tests for success, API errors, 408 clock skew, bad JSON

### M2 — live data

- [x] Wire the real provider into the UI
- [x] Merge API `inverterDay` with local samples
- [x] Cache fallback: the last stored snapshot renders when the cloud is down
- [x] Offline and error states in the header and diagnostics (cached fallback)
- [x] Re-discover inverter from the Diagnostics screen
- [ ] Manual refresh is implemented; verify it against a live account

### M3 — polish

- [x] Mouse hit testing for tabs, tiles and chart cursor
- [x] JSON and CSV export
- [x] Transparent theme option
- [x] `NO_COLOR` monochrome fallback; colour profile degrades through Lip Gloss
- [x] Seven chart styles (graph default, solid, area, box, quadrant, braille, ascii)
- [x] Solar area fill, source-mix stacked area, year heatmap and SOC arc
- [ ] Empty, loading and error states on every screen (partial)

### M4 — verification and release

- [x] README with build, configuration and key documentation
- [ ] Live field and sign verification with credentials or a JSON dump
- [ ] Fix any alias/sign mismatches found
- [ ] Golden screenshots in the README
- [ ] Tag a first version

## Non-goals

- Inverter control, charge schedules, export limits, firmware.
- Home Assistant, MQTT, Prometheus or web frontends.
- Other solar providers in this scope.
- Replacing `solis-kde-plasma-sensors`; it stays untouched.
