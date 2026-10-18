# Solis Monitor TUI — Research Notes

Context carried over from `../solis-kde-plasma-sensors/` (read on 2026-10-18),
plus the SolisCloud Platform API V2.0 document and community field dumps.
This file is background for the Go TUI; it is not a spec.

## 1. What the old project did

`solis-kde-plasma-sensors` is a Python 3 (stdlib only) daemon that turns
SolisCloud data into nine KDE System Monitor sensors.

```
SolisCloud REST API
      │  POST, HMAC-SHA1 signed, every 300 s
      ▼
solis2kde.py  (systemd --user service)
      │  atomic writes to /tmp/solis_*.txt
      ▼
ksystemstats_custom_sensors plugin (built from source)
      │  reads ~/.config/customsensorrc
      ▼
KDE Plasma System Monitor
```

Key files:

| File | Purpose |
|---|---|
| `solis2kde.py` | config parsing, request signing, fetch loop, field extraction, atomic file writes |
| `customsensorrc` | sensor id/name/file/unit definitions for the KDE plugin |
| `solis2kde.service` | systemd user unit, `Restart=on-failure`, `RestartSec=30` |
| `install.sh` | builds the KDE plugin, installs daemon, creates config |
| `README.md` | architecture, field map, troubleshooting |

Relevant behaviour worth keeping (or improving) in the TUI:

- **No third-party Python deps.** Config is a flat `key: value` YAML subset at
  `~/.config/solis2kde/config.yaml`, mode `0600`.
- **Inverter ID discovery** is separate from detail polling. The daemon calls
  `inverterList` once, caches the first inverter id, and re-discovers after five
  consecutive failures.
- **Retry policy**: failures are logged as warnings, then errors after three
  consecutive failures; the loop continues.
- **Atomic file writes** (`.tmp` + `rename`) so readers never see a partial value.
- **Missing fields become 0**, which hides "unknown" from the consumer. A TUI
  should represent unknown/stale data explicitly instead.
- **Grid magnitude is written without direction** because KDE sensors are
  scalar; direction is only logged. A TUI can and should show both magnitude and
  direction (import/export).
- Old project's fixed mappings (actual inverter: **S6-EH1P6K-L-PRO**):

| Metric | API field(s) | Scaling |
|---|---|---|
| Solar generation | `pac` | kW × 1000 → W |
| Grid (total, signed) | `psumOrgin` | W; `+` export, `−` import |
| Grid meter power | `acGridPowerA` | W; negative = import, old code took `abs()` |
| House load | `familyLoadPowerOrigin`, fallback `totalLoadPower` | W |
| Backup / essential load | `backupPowerA` | W |
| Smart / non-essential load | computed: `load − backup` | W |
| Battery SOC | `batteryCapacitySoc`, fallback `batteryPercent` | % |
| Energy today | `eToday` | kWh |
| Lifetime energy | `allEnergyOriginal`, fallback `eTotal` | kWh (`eTotal` may be MWh) |

## 2. SolisCloud API essentials

**Base URL:** `https://www.soliscloud.com:13333`

**Every request is `POST` with a JSON body.**

### Signing (from `solis2kde.py`, matches API doc V2.0)

```
date        = RFC 1123 GMT, e.g. "Fri, 26 Jul 2019 06:00:46 GMT"
body        = compact JSON, separators (",", ":")   ← must match byte-for-byte
contentMd5  = base64(md5(body))
stringToSign = METHOD + "\n" + contentMd5 + "\n"
             + "application/json" + "\n"
             + date + "\n"
             + path-only URI           e.g. "/v1/api/inverterDetail"
signature   = base64(HMAC-SHA1(apiSecret, stringToSign))
```

Headers sent:

```
Content-Type: application/json;charset=UTF-8
Date: <rfc1123>
Content-MD5: <base64 md5>
Authorization: API <apiId>:<signature>
```

Note the `Content-Type` header and the content type inside the signature string
differ (`;charset=UTF-8` vs plain `application/json`); the old code does this and
it works.

### Response envelope

```json
{ "success": true, "code": "0", "msg": "success", "data": { ... } }
```

`code == "0"` is success. `data` may be an object (detail endpoints) or a page
object `{page: {records: [...]}, total: ...}` (list endpoints).

### Constraints

- **Data refresh cadence: 5 minutes.** More frequent polling mostly returns the
  same sample. The old project used 300 s; its install template says 300 s is the
  free-tier minimum.
- **Rate limit: 2 requests/second** per endpoint (per the V2.0 document). Not a
  problem for a 5-minute poll, but history/backfill should be paced.
- **Clock skew > ±15 minutes → HTTP 408.** Signing depends on the `Date` header.
- **Units are not fixed.** Values come with companion `*Str` fields
  (`pacStr`, `eTotalStr`, `batteryPowerStr`, ...). The API doc states all values
  must be interpreted together with their unit. Power is usually kW, energy is
  usually kWh, but totals may be MWh.
- All monitoring data is pushed to the cloud by the datalogger; latency of one
  or two 5-minute cycles is normal.

### Useful endpoints

| Endpoint | Use |
|---|---|
| `/v1/api/inverterList` | discover inverter id/SN for a station (`stationId`, `pageNo`, `pageSize`) |
| `/v1/api/inverterDetail` | current snapshot: power, battery, energy, temperatures, PV strings, state |
| `/v1/api/inverterDetailList` | snapshot for all inverters in one call |
| `/v1/api/inverterDay` | 5-minute series for one day (`id` or `sn`, `money`, `time`, `timeZone`) |
| `/v1/api/inverterMonth` | daily series for a month |
| `/v1/api/inverterYear` | monthly series for a year |
| `/v1/api/stationDay` / `stationMonth` / `stationYear` | station-level equivalents |
| `/v1/api/alarmList` | device alarms (level 1 tip / 2 general / 3 emergency) |

`inverterDay` is the natural source for a "today" generation curve without
building a database. It returns timestamped points including `pac`, `eToday`,
`batteryCapacitySoc`, `batteryPower`, `familyLoadPower`, `bypassLoadPower`,
`pSum`, PV voltages/currents, `inverterTemperature`. A local sample store is
still useful for sub-5-minute smoothing and for history the API does not keep.

## 3. Field inventory for a storage inverter

The old project used only nine fields. The API exposes far more. Most promising
for a dashboard, based on the V2.0 detail schema and community response dumps:

### Power now

| Field | Unit | Meaning |
|---|---|---|
| `pac` | kW (`pacStr`) | PV generation |
| `power` | kW | installed/rated capacity context |
| `psum`, `psumOrgin`, `psumCal` | kW/W | total grid active power; sign convention `+` export, `−` import (verify on live data) |
| `acGridPowerA` | W | grid-side meter / CT reading |
| `familyLoadPower`, `familyLoadPowerOrigin`, `totalLoadPower` | kW | house load |
| `backupPowerA`, `bypassLoadPower` | kW/W | backup/essential circuits |
| `batteryPower` | kW | battery power; `batteryPowerZheng` / `batteryPowerFu` appear to carry the two directions (sign meaning to verify live) |
| `pvAndAcCoupledPower`, `totalAndSmartLoadPower` | kW | combined PV + AC-coupled, total + smart load |
| `generatorPower` | kW | generator input if present |
| `sscCurrentFlowMap` | object | `gridToLoad`, `batteryToLoad`, `batteryToGrid`, `pvToBattery` — ready-made flow split |

### Battery

| Field | Unit | Meaning |
|---|---|---|
| `batteryCapacitySoc` | % | state of charge |
| `batteryHealthSoh` | % | state of health |
| `batteryVoltage`, `storageBatteryVoltage` | V | pack voltage |
| `bstteryCurrent`, `storageBatteryCurrent` | A | current |
| `batteryTodayChargeEnergy` / `batteryTodayDischargeEnergy` | kWh | daily throughput |
| `batteryTotalChargeEnergy` / `batteryTotalDischargeEnergy` | kWh | lifetime throughput |
| `batteryState`, `batteryAlarm`, `batteryFailureInformation01/02` | codes | health/faults |
| `socDischargeSet`, `socChargingSet` | % | configured reserve/charge limits |
| `batteryType`, `batteryNum`, `batteryList` | | pack metadata |

### PV strings

`uPv1..uPv32` (V), `iPv1..iPv32` (A), `pow1..pow32` (W), plus `mpptUpvN`,
`mpptIpvN`, `mpptPowN`. Totals: `upvTotal`, `ipvTotal`, `powTotal`.
`dcInputType` encodes channel count as `value + 1`.

### Grid / AC

`uAc1..3` (V), `iAc1..3` (A), `fac` (Hz), `powerFactor`, `reactivePower`,
`apparentPower`, `aPhasePowerFactor` / `bPhase...` / `cPhase...`.

### Energy counters

| Field | Unit | Meaning |
|---|---|---|
| `eToday` | kWh | generated today |
| `eMonth`, `eYear`, `eTotal` | unit in `*Str` | generated this month/year/lifetime |
| `allEnergyOriginal` | kWh | lifetime generation, kWh-normalised |
| `homeLoadTodayEnergy` / `Month` / `Year` / `Total` | kWh | consumption |
| `gridPurchasedTodayEnergy` / `Total` | kWh | import |
| `gridSellTodayEnergy` / `Total` | kWh | export |
| `backupTodayEnergy` / `Total` | kWh | backup circuits |
| `homeGridTodayEnergy` / `Total` | kWh | grid-side house consumption |

### Status and context

`state` (1 online, 2 offline, 3 alarm), `currentState`, `alarmState`,
`faultCodeDesc` (e.g. "Generating"), `inverterTemperature`,
`inverterTemperature2`, `temp`/`tempName` (e.g. IGBT), `model`, `productModel`,
`machine`, `sn`, `stationName`, `timeStr`, `dataTimestamp`, `timeZoneStr`,
`version`, `batteryCDEnableSet`, `pLimitSet`.

## 4. Implications for the Go TUI

1. **Talk to the API directly.** The old `/tmp` files are an implementation
   detail of the KDE bridge and are not present on this machine (no
   `~/.config/solis2kde`, no `/tmp/solis_*`). Direct access gives battery power,
   direction, alarms, and history that the KDE path discarded.
2. **Cache last-good snapshot on disk** (e.g.
   `~/.local/state/solis-monitor/`) so the UI opens instantly and shows stale
   data with an age badge when offline. Keep a ring buffer of local samples for
   a live-ish sparkline between 5-minute cloud updates.
3. **Model units explicitly.** Store values as `float64` plus unit, or convert
   everything to W/kWh at the edge and keep the raw `*Str` for display. Never
   silently turn missing data into zero.
4. **Respect the cadence.** Default poll 300 s, configurable but warn below
   ~60 s. Fetch `inverterDay` on startup and then every ~5–10 minutes; merge
   with local samples.
5. **Sign convention is the main correctness risk.** `psumOrgin` and battery
   power signs must be verified against a live sample (e.g. at night with
   known import). Encode direction as a type (`Import`/`Export`/`Idle`) rather
   than a bare signed number.
6. **Align with the existing Go TUI stack.** The sibling project
   `../video-downloader-tui` uses Go 1.27, `charm.land/bubbletea/v2`,
   `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`, and
   `github.com/NimbleMarkets/ntcharts/v2`. Reusing that stack keeps look, key
   handling, and test patterns consistent.
7. **Layout candidates** for one screen:
   - header: station/inverter, online state, sample age, clock
   - energy-flow strip: PV → house/battery/grid with directional arrows
   - live power tiles: solar, load, grid (import/export), battery (charge/discharge)
   - battery gauge: SOC, power, today's charge/discharge
   - today curve: `inverterDay` `pac` over time, with "now" marker
   - footer: today/month/lifetime energy, temperature, alarms
8. **Config:** either reuse `~/.config/solis2kde/config.yaml` (keeps one secret
   file for both projects) or introduce
   `~/.config/solis-monitor/config.toml` with `api_key`, `api_secret`,
   `station_id`, `inverter_id` optional, `poll_interval`, `timezone`. Support
   env overrides so the TUI can be tested without a config file.

## 5. Open questions before implementation

- Which config path/format: reuse the old `solis2kde/config.yaml`, or a new
  `solis-monitor` config?
- Confirm inverter model and whether `psumOrgin` / `acGridPowerA` /
  `familyLoadPowerOrigin` / `backupPowerA` appear on live data (a `--discover`
  dump from the real account would settle field names, signs, and units).
- Should history be local-only (SQLite/JSONL) or fetched from
  `inverterDay`/`inverterMonth` on demand?
- Which views matter: single dashboard only, or tabs for today / month /
  lifetime / alarms?
- Any preference for a color/visual direction beyond "nice looking"?
