# Decisions

Architecture decisions are recorded here as short ADRs. Newest last. Each
entry states the context, the decision and its consequences.

## ADR-0001 — Go + Bubble Tea v2 stack

**Context.** The sibling project `video-downloader-tui` uses Go 1.27 with
Bubble Tea v2, Lip Gloss v2 and Bubbles v2. The user wants a polished TUI and
prefers that look and feel.

**Decision.** Use the same stack: `charm.land/bubbletea/v2`,
`charm.land/lipgloss/v2`, `charm.land/bubbles/v2`, plus
`github.com/charmbracelet/x/ansi` for width handling.

**Consequences.** Consistent key handling, layout and testing patterns. We
inherit v2's `tea.View` API, including `BackgroundColor` for the painted canvas.

## ADR-0002 — Direct SolisCloud access, read-only

**Context.** The old project writes values to `/tmp` for a KDE plugin. That
path discards direction, battery power, PV strings and alarms.

**Decision.** Talk to the SolisCloud REST API directly and keep the application
strictly read-only. Do not read the old `/tmp` files and do not use device
control endpoints.

**Consequences.** The TUI is independent of the KDE daemon and can show the
full snapshot. It needs its own credentials and must respect the API cadence.

## ADR-0003 — SQLite via `modernc.org/sqlite`

**Context.** The user chose SQLite for local history. A cgo-free build keeps
`go build` and release builds simple across systems.

**Decision.** Use `modernc.org/sqlite` through `database/sql`.

**Consequences.** Larger dependency and binary than a cgo driver, but no C
toolchain requirement. Schema migrations use `PRAGMA user_version`.

## ADR-0004 — Provider interface, Solis first

**Context.** The user may extend the app to other solar APIs later.

**Decision.** Define `internal/provider.Source` over the domain types. Implement
`internal/provider/solis` now and `internal/provider/demo` for simulation. The
UI imports only `internal/provider`.

**Consequences.** A future provider is additive. Do not build abstractions that
only make sense for hypothetical providers.

## ADR-0005 — Painted dark canvas

**Context.** The user chose a solar dark palette and wants the app to look the
same on any terminal.

**Decision.** Set `tea.View.BackgroundColor` to the theme background and
`ForegroundColor` to the theme text colour. Provide a `transparent` theme as an
escape hatch.

**Consequences.** The palette is stable in screenshots and across terminal
themes. A terminal that does not support background colour falls back to its
own background; the design must remain readable there.

## ADR-0006 — Custom chart renderers

**Context.** `ntcharts` line charts are single-series. The Today screen needs
multiple series on shared axes plus a cursor, and the sparklines/bars need to
match the theme exactly.

**Decision.** Implement a small chart package in `internal/ui/components`:
braille multi-series line chart, block bar chart, block sparkline. Do not add
`ntcharts` in M0.

**Consequences.** Full control over colour, axes, cursor and responsive sizes.
More code to test. If the custom renderer becomes a maintenance burden, revisit
against `ntcharts`.

## ADR-0007 — Naming and paths

**Context.** The user named the app `solar-monitor-go` and wants room for
other APIs.

**Decision.** Module/repo and display title are `solar-monitor-go`; the binary
is `solar-monitor`; config and state directories are `solar-monitor`.

**Consequences.** User-facing commands stay short while the project name stays
distinctive. The env prefix is `monitor_`.

## ADR-0008 — API income and PKR

**Context.** SolisCloud returns income fields with a currency marker that may
not be PKR. The user wants earnings in PKR.

**Decision.** Display API income only when its currency matches the configured
currency. Otherwise compute earnings from configured import/export tariffs and
label the source in the UI.

**Consequences.** Never present a foreign-currency number as PKR. When no
tariff is configured, earnings show `—` rather than a misleading zero.

## ADR-0009 — Local history behind a monitor service

**Context.** The app should keep history forever and still show something useful
when SolisCloud is unreachable. The UI must not know about SQLite or about
Solis-specific caching.

**Decision.** Keep persistence in `internal/store` and wrap the provider in
`internal/monitor.Service`, which implements `provider.Source`. The service
records every snapshot, merges local samples into today's curve, falls back to
stored data on provider errors, and exposes a `Seed` for an instant first frame.

**Consequences.** The UI stays provider-agnostic. Store statistics reach the
Diagnostics screen through a small `Options.Stats` callback that returns a
UI-owned summary, so `ui` never imports `store`.

## ADR-0010 — Injectable clocks

**Context.** The demo provider advances a simulated clock and the UI renders
ages, countdowns and dates. Golden tests cannot depend on the wall clock.

**Decision.** `demo.Options.Clock` and an internal `Model.clock` accept a
function returning `time.Time`. `fmtAgeFrom(now, t)` and
`fmtCountdown(now, t)` take an explicit now. Production code passes `time.Now`.

**Consequences.** Golden frames are stable to the second and reproducible in CI.
Any new time-dependent UI must take its now from `m.now()` rather than
`time.Now()`.


## ADR-0011 — Fixed-day charts, axes and framed panels

**Context.** The first Live chart had no axes, so "now" drifted across the
width, and the Today screen mixed summary, legend and chart without structure.

**Decision.** Charts always draw a y axis and an x axis, and every chart is
framed with a titled border. Day charts use a fixed 00:00-23:59 domain by
default, filled from five-minute slots, so only the plotted lines grow.
`display.day_window = "auto"` and the `x` key switch to fit-to-samples. Today,
History's day view and the Battery SOC chart are always fixed-day. Summaries
live in their own bordered panel with one idea per line.

**Consequences.** Users can read the time of day and power level directly from
the chart. Fixed-day charts contain 288 slots regardless of how many samples
arrived, which keeps the x axis stable. The bar charts gained the same frame and
y axis, so History reads consistently. Spacing and height budgets now matter:
at 24 rows the tile sparkline row is dropped so the chart keeps at least five
plot rows.

## ADR-0012 — Solid half-block line charts

**Context.** The braille line charts were accurate but sparse: at 5-8 plot rows
the dots read as broken dashes and were hard to follow at a glance.

**Decision.** `LineOptions.Style` defaults to `ChartSolid`, which fills each
column with the half-block range its samples cover (`▀`, `▄`, `█`). This gives
two vertical sub-rows per cell, keeps steep changes connected, and still lets
each series keep its colour. `ChartBox` offers thin box-drawing lines with
junction merging, and `ChartBraille` keeps the original dot renderer.

**Consequences.** Lines look solid and readable in shallow charts. Cells where
several series overlap take the first series' colour, so crossings can briefly
hide a later series; the legend and cursor values carry that information. The
golden frames encode the style, so a future style change is a deliberate update.

## ADR-0013 — Chart style set, area fill, source mix, heatmap and SOC arc

**Context.** Braille, solid, box and ntcharts graph lines all have different
readability tradeoffs. Users also need to see generation shape, the source mix,
a year at a glance and the battery level without reading numbers.

**Decision.** Keep `solid` as the default and expose `area`, `box`, `quadrant`,
`braille`, `ascii` and `graph` through `display.chart_style` and
`--chart-style`, defaulting to `braille`. `solid`, `area` and `graph` fill the solar area down
to the zero baseline; braille leaves it clear for readability.
`v` switches the day chart to a stacked source-mix area. History's year view is
a calendar heatmap; month keeps bars. The Battery screen uses a semicircular
SOC arc above the detailed values.

**Consequences.** The chart component now has one style switch plus optional
fill, and the UI has one view switch. The year heatmap needs daily data, so year
mode makes one `inverterMonth` call per month (paced), whereas month mode keeps
its single call. Image-free rendering stays portable and dependency-light;
`graph` adds `NimbleMarkets/ntcharts` for its line renderer.

## ADR-0014 — Readability pass for tall braille charts

**Context.** On a wide, tall terminal the braille chart had only three y labels
and the solar area fill sat over the thin dots, turning the plot into a solid
brown mass.

**Decision.** Braille no longer draws the area fill. Tall charts (eight or more
plot rows) get five y ticks and a subtle dot grid at the tick rows and x-label
columns. `display.chart_smoothing` defaults to a three-sample centred moving
average; 1 disables it.

**Consequences.** The dots stay distinct, values are easier to align with the
axes, and cloud spikes are softened without hiding the overall shape. Raw data
is still available by setting `chart_smoothing = 1`.

## ADR-0015 — Overlap colour blending

**Context.** In a small braille chart two series can occupy the same cell. The
renderer used the first series' colour, so one line appeared to change colour
where another crossed or nearly crossed it.

**Decision.** When two series share a braille or quadrant cell, blend their
colours at draw time so the overlap reads as an overlap.

**Consequences.** Overlaps become a neutral blend instead of stealing a series
colour. The underlying values are untouched.

## ADR-0016 — Responsive year view and cache-first daily data

**Context.** At 80 columns the year heatmap is a small block of shades, and
fetching twelve months on every visit made it slow.

**Decision.** Use monthly bars for the year on terminals under 110×28 and the
heatmap above that. Daily rows are read from SQLite first; only missing or
incomplete months are requested. `provider.YearDaysSource` lets the monitor
service own that policy, with the demo provider and the UI fallback sharing the
same interface.

**Consequences.** Small terminals get a readable year and make no daily
requests. Wide terminals pay the twelve calls once, then read from disk. The
year heatmap only loads when it is actually shown, including after a resize.

## ADR-0017 — Alarm resilience when the station id is not usable

**Context.** The account's configured `station_id` is not a station the API
accepts for `alarmList`: sending it returns the generic API code 1
("Communication error"). The call succeeds once `stationId` is dropped and the
request is scoped by `alarmDeviceSn`, or account-wide.

**Decision.** Never send `stationId` with `alarmList`. Scope by
`alarmDeviceSn` when the inverter SN is known; on API code 1, retry once
account-wide and filter the results to that SN locally. Alarm failures retry
after one minute instead of waiting a full alarm interval, and the empty state
distinguishes an error (red `!`) from "no alarms" (green check).

**Consequences.** Alarms work on accounts whose station id is missing or
unusable, and a transient code 1 self-heals. Account-wide results are filtered
so another inverter's alarms are not shown. `ensureID` also prefers a
configured serial number over discovery.
