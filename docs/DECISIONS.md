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

