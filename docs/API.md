# SolisCloud API reference

Authoritative notes for `internal/provider/solis`. Sources: SolisCloud Platform
API Document V2.0 and the working implementation in the older
`../solis-kde-plasma-sensors` project.

## Connection

- Base URL: `https://www.soliscloud.com:13333`
- All requests are `POST` with a JSON body.
- All data refreshes every 5 minutes in the cloud.
- Rate limit is 2 requests/second per endpoint.
- A local clock more than 15 minutes off returns HTTP 408.

## Signing

```
date         = RFC 1123 in GMT, e.g. "Fri, 26 Jul 2019 06:00:46 GMT"
body         = compact JSON, separators (",", ":")
contentMD5   = base64(md5(body))
stringToSign = METHOD + "\n" + contentMD5 + "\n"
             + "application/json" + "\n"
             + date + "\n"
             + path-only URI                 e.g. "/v1/api/inverterDetail"
signature    = base64(HMAC-SHA1(apiSecret, stringToSign))
```

Headers:

```
Content-Type: application/json;charset=UTF-8
Date: <rfc1123>
Content-MD5: <base64 md5>
Authorization: API <apiId>:<signature>
```

The content type in the signature string is plain `application/json` while the
header carries `;charset=UTF-8`. Both are required exactly as shown.

## Response envelope

```json
{ "success": true, "code": "0", "msg": "success", "data": { } }
```

`code == "0"` means success. `code` may arrive as a string or a number; parse
both. `data` is an object for detail endpoints and an array or page object for
list endpoints.

List endpoints return `data.page.records` (with a fallback to `data.records`).

## Endpoints used

| Endpoint | Purpose | Body |
|---|---|---|
| `/v1/api/inverterList` | discover inverter id/SN | `{"id": stationId, "pageNo": 1, "pageSize": 10}` |
| `/v1/api/inverterDetail` | current snapshot | `{"id": inverterID}` |
| `/v1/api/inverterDay` | 5-minute series for a day | `{"id": id, "money": currency, "time": "YYYY-MM-DD", "timeZone": offsetHours}` |
| `/v1/api/inverterMonth` | daily series for a month | `{"id": id, "money": currency, "month": "YYYY-MM", "timeZone": offsetHours}` |
| `/v1/api/inverterYear` | monthly series for a year | `{"id": id, "money": currency, "year": "YYYY", "timeZone": offsetHours}` |
| `/v1/api/alarmList` | device alarms | `{"pageNo": "1", "pageSize": "100", "alarmDeviceSn": sn, "alarmBeginTime": "YYYY-MM-DD", "alarmEndTime": "YYYY-MM-DD"}` |

`id` may be either the numeric inverter id or the serial number, but not both
empty.

For `alarmList`, prefer `alarmDeviceSn` over `stationId`. A station id the
account cannot see returns API code 1 ("Communication error"); the client
retries account-wide and filters by serial number. `timeZone` is an integer hour offset; the examples in the vendor
document use both `8` and `"8"`.

## Units

Every value is accompanied by a `*Str` unit field: `pacStr`, `eTotalStr`,
`batteryPowerStr`, `eTodayStr` and so on. The vendor document states that
values must be interpreted together with their unit.

Observed conventions:

- Power values (`pac`, `psum`, `familyLoadPower`, `batteryPower`) are usually
  kW. Some legacy fields (`psumOrgin`, `acGridPowerA`, `familyLoadPowerOrigin`,
  `backupPowerA`) arrive as watts.
- `eToday`, `eMonth`, `eYear` are usually kWh. `eTotal` may be MWh;
  `allEnergyOriginal` is kWh.
- `dataTimestamp` is UTC+8; `timeStr` is already converted to the device time
  zone. Prefer `timeStr` when present.

`internal/provider/solis/normalize.go` converts to W and kWh at ingest and
keeps the raw snapshot for diagnostics.

## Field reference

### Power and flow

| Field | Notes |
|---|---|
| `pac` | PV generation, kW |
| `psum`, `psumOrgin`, `psumCal` | grid active power; older docs use `psumOrgin` in W |
| `acGridPowerA` | grid-side meter reading, W |
| `familyLoadPower`, `familyLoadPowerOrigin`, `totalLoadPower` | house load |
| `backupPowerA`, `bypassLoadPower` | backup / essential circuits |
| `batteryPower` | battery power; sign verification pending |
| `batteryPowerZheng`, `batteryPowerFu` | the two battery directions |
| `sscCurrentFlowMap` | `gridToLoad`, `batteryToLoad`, `batteryToGrid`, `pvToBattery` |
| `generatorPower` | generator input when present |

### Battery

`batteryCapacitySoc`, `batteryHealthSoh`, `batteryVoltage`,
`storageBatteryVoltage`, `bstteryCurrent`, `storageBatteryCurrent`,
`batteryTodayChargeEnergy`, `batteryTodayDischargeEnergy`,
`batteryTotalChargeEnergy`, `batteryTotalDischargeEnergy`,
`socDischargeSet`, `socChargingSet`, `batteryState`, `batteryAlarm`,
`batteryFailureInformation01`, `batteryFailureInformation02`.

### PV strings

`uPv1..uPv32` (V), `iPv1..iPv32` (A), `pow1..pow32` (W), `mpptUpvN`,
`mpptIpvN`, `mpptPowN`, and the totals `upvTotal`, `ipvTotal`, `powTotal`.
`dcInputType` encodes the channel count as `value + 1`.

### Grid and AC

`uAc1..3`, `iAc1..3`, `fac`, `powerFactor`, `reactivePower`, `apparentPower`,
`aPhasePowerFactor`, `bPhasePowerFactor`, `cPhasePowerFactor`.

### Energy counters

`eToday`, `eMonth`, `eYear`, `eTotal`, `allEnergyOriginal`,
`homeLoadTodayEnergy`, `homeLoadTotalEnergy`, `gridPurchasedTodayEnergy`,
`gridPurchasedTotalEnergy`, `gridSellTodayEnergy`, `gridSellTotalEnergy`,
`backupTodayEnergy`, `backupTotalEnergy`.

### Income

`dayInCome`, `monthInCome`, `yearInCome`, `allInCome`, and the `money` /
`moneyStr` pair. Only used when the currency matches the configured currency.

### Status

`state` (1 online, 2 offline, 3 alarm), `currentState`, `alarmState`,
`faultCodeDesc`, `inverterTemperature`, `inverterTemperature2`, `tempName`,
`model`, `productModel`, `machine`, `sn`, `stationName`, `timeStr`,
`dataTimestamp`, `timeZoneStr`, `version`.

## Sign conventions

- Grid: positive `psumOrgin` is export, negative is import. To be verified on
  live data.
- Battery: positive `batteryPower` is assumed to be charging. To be verified.
- House load from `familyLoadPowerOrigin` is the total load, with
  `backupPowerA` a subset of it. Smart/non-essential load is
  `total − backup`.

When no flow map is present, `internal/monitor` derives flows and runs a
balance check:
`solar + discharge + import ≈ load + charge + export`.

## Error handling

| Condition | Behaviour |
|---|---|
| HTTP 408 | Surface a clock-skew message; suggest checking system time |
| Non-zero `code` | Surface `msg` and `code`; do not retry immediately |
| Network or timeout | Exponential backoff: 30 s, 1 m, 2 m, 5 m, then hold |
| Bad JSON | Report as a provider error, keep the last good snapshot |

## Discovery

`--discover` prints the raw `inverterDetail` payload plus a normalised summary
so aliases, units and signs can be confirmed against the real account.
