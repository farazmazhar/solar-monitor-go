# Status

Updated: 2026-10-18
Current milestone: M0 — foundation and visual proof
Overall state: starting. Documentation scaffold and repository setup in progress.

## In progress

- [ ] M0.0 — repo initialization, AGENTS.md, docs scaffold, Makefile, first commit

## Next

- [ ] M0.1 — Go scaffold, config loader, provider interface, demo provider
- [ ] M0.2 — theme, tab shell, Live screen at 80×24, golden render tests
- [ ] M0.3 — Today, History, Battery, PV, Alarms, Diagnostics screens
- [ ] M0.4 — responsive 60×20 / 100×30, help overlay, motion, mouse

## Blocked

- [ ] Live SolisCloud field and sign verification — no credentials on this
      machine yet. Build and test against the demo provider; use `--discover`
      or a saved `inverterDetail` JSON when available.

## Done

- [x] Read the old `solis-kde-plasma-sensors` project and captured research
- [x] Agreed the product, UI direction, stack and scope with the user
- [x] Moved research into `docs/RESEARCH.md`
- [x] Added `AGENTS.md` and the documentation skeleton

## Verification

- `make check`: not run yet (no Go code yet)
- Manual UI check: not run yet
- Live API check: cannot run without credentials

## Notes

- App name is `solar-moonitor-go` as specified by the user; binary is
  `solar-moonitor`, config and state dirs omit the `-go` suffix.
- Currency is PKR (₨).
- Raw samples are retained forever; raw JSON snapshots are pruned after 30 days.
- Scope is strictly read-only.
