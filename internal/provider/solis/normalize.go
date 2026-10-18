package solis

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

// NormalizeOptions controls unit and sign conversion for one payload.
type NormalizeOptions struct {
	Now             time.Time
	Location        *time.Location
	Currency        string
	GridPositive    string
	BatteryPositive string
	Source          string
}

type candidate struct {
	name        string
	defaultUnit string
}

func (o NormalizeOptions) loc() *time.Location {
	if o.Location == nil {
		return time.Local
	}
	return o.Location
}

func (o NormalizeOptions) now() time.Time {
	if o.Now.IsZero() {
		return time.Now()
	}
	return o.Now
}

// NormalizeSnapshot converts one inverterDetail payload into the domain model.
func NormalizeSnapshot(raw json.RawMessage, opts NormalizeOptions) (provider.Snapshot, error) {
	m, err := decodeMap(raw)
	if err != nil {
		return provider.Snapshot{}, fmt.Errorf("solis: decode detail: %w", err)
	}

	snap := provider.Snapshot{
		FetchedAt: opts.now(),
		DataAt:    parseDataTime(m, opts.loc(), opts.now()),
		Source:    opts.Source,
		Raw:       raw,
	}

	snap.Station = textField(m, "stationName")
	snap.Model = firstText(m, "model", "productModel", "machine")
	snap.SN = textField(m, "sn")
	snap.State = parseState(m)
	snap.Currency = firstText(m, "money", "moneyStr")
	if snap.Currency == "" {
		snap.Currency = opts.Currency
	}

	snap.SolarW = powerField(m, candidate{"pac", "kW"})
	snap.LoadW = powerField(m,
		candidate{"familyLoadPowerOrigin", "W"},
		candidate{"familyLoadPower", "kW"},
		candidate{"totalLoadPower", "kW"},
	)
	snap.BackupW = powerField(m,
		candidate{"backupPowerA", "W"},
		candidate{"bypassLoadPower", "kW"},
	)
	snap.GridW = powerField(m,
		candidate{"psumOrgin", "W"},
		candidate{"psum", "kW"},
		candidate{"psumV2", "kW"},
		candidate{"acGridPowerA", "W"},
	)
	snap.BatteryW = powerField(m,
		candidate{"batteryPower", "kW"},
		candidate{"batteryPowerBms", "kW"},
	)
	snap.BatteryV = numberField(m, "storageBatteryVoltage", "batteryVoltage")
	snap.BatteryA = numberField(m, "storageBatteryCurrent", "bstteryCurrent")
	if snap.BatteryW == nil && snap.BatteryV != nil && snap.BatteryA != nil {
		snap.BatteryW = provider.Float(*snap.BatteryV * *snap.BatteryA)
	}

	snap.SOC = numberField(m, "batteryCapacitySoc", "batteryPercent")
	snap.SOH = numberField(m, "batteryHealthSoh")
	snap.TempC = numberField(m, "inverterTemperature")
	snap.GridHz = numberField(m, "fac")
	snap.GridV = numberField(m, "uAc1")
	snap.ReserveSOC = numberField(m, "socDischargeSet")
	snap.ChargeSOC = numberField(m, "socChargingSet")

	applySigns(&snap, opts)

	snap.TodayKWh = energyField(m,
		candidate{"eToday", "kWh"},
	)
	snap.MonthKWh = energyField(m, candidate{"eMonth", "kWh"})
	snap.YearKWh = energyField(m, candidate{"eYear", "kWh"})
	snap.TotalKWh = energyField(m,
		candidate{"allEnergyOriginal", "kWh"},
		candidate{"eTotal", "kWh"},
	)
	snap.LoadTodayKWh = energyField(m, candidate{"homeLoadTodayEnergy", "kWh"})
	snap.ImportTodayKWh = energyField(m, candidate{"gridPurchasedTodayEnergy", "kWh"})
	snap.ExportTodayKWh = energyField(m, candidate{"gridSellTodayEnergy", "kWh"})
	snap.ChargeTodayKWh = energyField(m, candidate{"batteryTodayChargeEnergy", "kWh"})
	snap.DischargeTodayKWh = energyField(m, candidate{"batteryTodayDischargeEnergy", "kWh"})
	snap.ChargeTotalKWh = energyField(m, candidate{"batteryTotalChargeEnergy", "kWh"})
	snap.DischargeTotalKWh = energyField(m, candidate{"batteryTotalDischargeEnergy", "kWh"})

	snap.IncomeDay = numberField(m, "dayInCome")
	snap.IncomeMonth = numberField(m, "monthInCome")
	snap.IncomeYear = numberField(m, "yearInCome")
	snap.IncomeTotal = numberField(m, "allInCome")

	snap.PVStrings = pvStrings(m)
	snap.Flow = flowForSnapshot(m, snap)
	snap.Faults = faults(m)
	return snap, nil
}

// NormalizeDay converts an inverterDay payload into samples.
func NormalizeDay(raw json.RawMessage, loc *time.Location) ([]provider.Sample, error) {
	if loc == nil {
		loc = time.Local
	}
	rows, err := decodeArray(raw)
	if err != nil {
		return nil, fmt.Errorf("solis: decode day: %w", err)
	}
	out := make([]provider.Sample, 0, len(rows))
	for _, m := range rows {
		ts := parseDataTime(m, loc, time.Time{})
		if ts.IsZero() {
			continue
		}
		solar := powerField(m, candidate{"pac", "kW"})
		load := powerField(m,
			candidate{"familyLoadPower", "kW"},
			candidate{"totalLoadPower", "kW"},
		)
		grid := powerField(m,
			candidate{"psum", "kW"},
			candidate{"psumV2", "kW"},
			candidate{"psumOrgin", "W"},
		)
		bat := powerField(m, candidate{"batteryPower", "kW"})
		s := provider.Sample{
			TS:       ts,
			SolarW:   solar,
			LoadW:    load,
			GridW:    grid,
			BatteryW: bat,
			SOC:      numberField(m, "batteryCapacitySoc"),
		}
		if grid != nil {
			if *grid >= 0 {
				s.ExportW = provider.Float(*grid)
				s.ImportW = provider.Float(0)
			} else {
				s.ImportW = provider.Float(-*grid)
				s.ExportW = provider.Float(0)
			}
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TS.Before(out[j].TS) })
	return out, nil
}

// NormalizeMonth converts an inverterMonth payload into daily energy rows.
func NormalizeMonth(raw json.RawMessage, loc *time.Location, fallbackCurrency string) ([]provider.DayEnergy, error) {
	return normalizeEnergyRows(raw, loc, fallbackCurrency, "2006-01-02",
		func(t time.Time, row energyRow, currency string) provider.DayEnergy {
			return provider.DayEnergy{
				Date:          t,
				GeneratedKWh:  row.generated,
				ConsumedKWh:   row.consumed,
				ImportedKWh:   row.imported,
				ExportedKWh:   row.exported,
				ChargedKWh:    row.charged,
				DischargedKWh: row.discharged,
				Income:        row.income,
				Currency:      currency,
			}
		},
		func(r provider.DayEnergy) time.Time { return r.Date },
	)
}

// NormalizeYear converts an inverterYear payload into monthly energy rows.
func NormalizeYear(raw json.RawMessage, loc *time.Location, fallbackCurrency string) ([]provider.MonthEnergy, error) {
	return normalizeEnergyRows(raw, loc, fallbackCurrency, "2006-01",
		func(t time.Time, row energyRow, currency string) provider.MonthEnergy {
			return provider.MonthEnergy{
				Month:         t,
				GeneratedKWh:  row.generated,
				ConsumedKWh:   row.consumed,
				ImportedKWh:   row.imported,
				ExportedKWh:   row.exported,
				ChargedKWh:    row.charged,
				DischargedKWh: row.discharged,
				Income:        row.income,
				Currency:      currency,
			}
		},
		func(r provider.MonthEnergy) time.Time { return r.Month },
	)
}

type energyRow struct {
	generated, consumed, imported, exported, charged, discharged, income *float64
}

func normalizeEnergyRows[T any](
	raw json.RawMessage,
	loc *time.Location,
	fallbackCurrency, layout string,
	build func(time.Time, energyRow, string) T,
	at func(T) time.Time,
) ([]T, error) {
	if loc == nil {
		loc = time.Local
	}
	arr, err := decodeArray(raw)
	if err != nil {
		return nil, fmt.Errorf("solis: decode energy rows: %w", err)
	}
	out := make([]T, 0, len(arr))
	for _, m := range arr {
		t := parseDateField(m, loc, layout)
		if t.IsZero() {
			continue
		}
		row := energyRow{
			generated:  energyField(m, candidate{"energy", "kWh"}),
			consumed:   energyField(m, candidate{"homeLoadEnergy", "kWh"}),
			imported:   energyField(m, candidate{"gridPurchasedEnergy", "kWh"}),
			exported:   energyField(m, candidate{"gridSellEnergy", "kWh"}),
			charged:    energyField(m, candidate{"batteryChargeEnergy", "kWh"}),
			discharged: energyField(m, candidate{"batteryDischargeEnergy", "kWh"}),
			income:     numberField(m, "money"),
		}
		currency := firstText(m, "moneyStr")
		if currency == "" {
			currency = fallbackCurrency
		}
		out = append(out, build(t, row, currency))
	}
	sort.Slice(out, func(i, j int) bool { return at(out[i]).Before(at(out[j])) })
	return out, nil
}

// NormalizeAlarms converts an alarmList payload into alarms.
func NormalizeAlarms(raw json.RawMessage, loc *time.Location) ([]provider.Alarm, error) {
	if loc == nil {
		loc = time.Local
	}
	var payload struct {
		Page struct {
			Records []map[string]any `json:"records"`
		} `json:"page"`
		Records []map[string]any `json:"records"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("solis: decode alarms: %w", err)
	}
	rows := payload.Page.Records
	if len(rows) == 0 {
		rows = payload.Records
	}
	out := make([]provider.Alarm, 0, len(rows))
	for _, m := range rows {
		a := provider.Alarm{
			ID:      firstText(m, "id", "alarmId"),
			Code:    textField(m, "alarmCode"),
			Message: textField(m, "alarmMsg"),
			Advice:  textField(m, "advice"),
			State:   textField(m, "state"),
		}
		if lvl := numberField(m, "alarmLevel"); lvl != nil {
			a.Level = int(*lvl)
		}
		a.Begin = parseMillisField(m, "alarmBeginTime", loc)
		a.End = parseMillisField(m, "alarmEndTime", loc)
		a.Active = a.End.IsZero() || a.State == "0"
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Begin.After(out[j].Begin) })
	return out, nil
}

func applySigns(s *provider.Snapshot, opts NormalizeOptions) {
	if s.GridW != nil {
		if opts.GridPositive == "import" {
			*s.GridW = -*s.GridW
		}
		if *s.GridW >= 0 {
			s.GridExportW = provider.Float(*s.GridW)
			s.GridImportW = provider.Float(0)
		} else {
			s.GridImportW = provider.Float(-*s.GridW)
			s.GridExportW = provider.Float(0)
		}
	}
	if s.BatteryW != nil && opts.BatteryPositive == "discharge" {
		*s.BatteryW = -*s.BatteryW
	}
}

func flowForSnapshot(m map[string]any, s provider.Snapshot) provider.Flow {
	if f, ok := flowFromMap(m); ok {
		return f
	}
	return deriveFlow(
		provider.Conv(s.SolarW),
		provider.Conv(s.LoadW),
		provider.Conv(s.GridW),
		provider.Conv(s.BatteryW),
	)
}

func flowFromMap(m map[string]any) (provider.Flow, bool) {
	v, ok := find(m, "sscCurrentFlowMap")
	if !ok {
		return provider.Flow{}, false
	}
	obj, ok := v.(map[string]any)
	if !ok {
		return provider.Flow{}, false
	}
	get := func(key string) (float64, bool) {
		raw, ok := find(obj, key)
		if !ok {
			return 0, false
		}
		f, ok := toFloat(raw)
		return f * 1000, ok
	}
	pvBat, ok1 := get("pvToBattery")
	batHouse, ok2 := get("batteryToLoad")
	batGrid, ok3 := get("batteryToGrid")
	gridHouse, ok4 := get("gridToLoad")
	if !(ok1 || ok2 || ok3 || ok4) {
		return provider.Flow{}, false
	}
	f := provider.Flow{
		PVToBattery:    math.Max(0, pvBat),
		BatteryToHouse: math.Max(0, batHouse),
		BatteryToGrid:  math.Max(0, batGrid),
		GridToHouse:    math.Max(0, gridHouse),
		Balanced:       true,
	}
	return f, true
}

// deriveFlow turns signed readings into a displayable split.
func deriveFlow(solar, load, grid, battery float64) provider.Flow {
	f := provider.Flow{Derived: true}
	if solar >= load {
		f.PVToHouse = load
		surplus := solar - load
		charge := math.Max(0, battery)
		f.PVToBattery = math.Min(surplus, charge)
		f.PVToGrid = surplus - f.PVToBattery
		if charge > surplus {
			f.GridToBattery = charge - surplus
		}
	} else {
		f.PVToHouse = solar
		deficit := load - solar
		discharge := math.Max(0, -battery)
		f.BatteryToHouse = math.Min(deficit, discharge)
		f.GridToHouse = deficit - f.BatteryToHouse
		if discharge > deficit {
			f.BatteryToGrid = discharge - deficit
		}
	}

	lhs := solar + math.Max(0, -grid) + math.Max(0, -battery)
	rhs := load + math.Max(0, battery) + math.Max(0, grid)
	diff := math.Abs(lhs - rhs)
	tol := math.Max(50, 0.05*math.Max(lhs, rhs))
	f.Balanced = diff <= tol
	return f
}

func parseState(m map[string]any) provider.State {
	if v := numberField(m, "state"); v != nil {
		switch int(*v) {
		case 1:
			return provider.StateOnline
		case 2:
			return provider.StateOffline
		case 3:
			return provider.StateAlarm
		}
	}
	switch strings.ToLower(textField(m, "currentState")) {
	case "online", "1":
		return provider.StateOnline
	case "offline", "2":
		return provider.StateOffline
	case "alarm", "3":
		return provider.StateAlarm
	}
	return provider.StateUnknown
}

func faults(m map[string]any) []string {
	var out []string
	for _, key := range []string{"batteryFailureInformation01", "batteryFailureInformation02", "batteryAlarm"} {
		if v := textField(m, key); v != "" && v != "0" && v != "none" {
			out = append(out, fmt.Sprintf("%s: %s", key, v))
		}
	}
	return out
}

func pvStrings(m map[string]any) []provider.PVString {
	count := 0
	if v := numberField(m, "dcInputType"); v != nil {
		count = int(*v) + 1
	}
	var out []provider.PVString
	emptyRun := 0
	for i := 1; i <= 32; i++ {
		suffix := strconv.Itoa(i)
		power := powerField(m,
			candidate{"pow" + suffix, "W"},
			candidate{"mpptPow" + suffix, "W"},
		)
		volt := numberField(m, "mpptUpv"+suffix, "uPv"+suffix)
		amp := numberField(m, "mpptIpv"+suffix, "iPv"+suffix)
		if power == nil && volt == nil && amp == nil {
			emptyRun++
			if emptyRun >= 2 && (count == 0 || i > count) {
				break
			}
			continue
		}
		emptyRun = 0
		if power == nil && volt != nil && amp != nil {
			power = provider.Float(*volt * *amp)
		}
		if (power != nil && *power > 0) || (volt != nil && *volt > 0) || (amp != nil && *amp > 0) {
			out = append(out, provider.PVString{Index: i, Voltage: volt, Current: amp, Power: power})
		}
	}
	if count > 0 && len(out) == 0 {
		for i := 1; i <= count; i++ {
			out = append(out, provider.PVString{Index: i})
		}
	}
	return out
}

func parseDataTime(m map[string]any, loc *time.Location, fallback time.Time) time.Time {
	if s := textField(m, "timeStr"); s != "" {
		for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339, "2006-01-02"} {
			if t, err := time.ParseInLocation(layout, s, loc); err == nil {
				return t
			}
		}
	}
	if t := parseMillisField(m, "dataTimestamp", loc); !t.IsZero() {
		return t
	}
	if t := parseMillisField(m, "dataTimestampStr", loc); !t.IsZero() {
		return t
	}
	return fallback
}

func parseDateField(m map[string]any, loc *time.Location, layout string) time.Time {
	if s := textField(m, "dateStr"); s != "" {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t
		}
	}
	if t := parseMillisField(m, "date", loc); !t.IsZero() {
		return t
	}
	return time.Time{}
}

func parseMillisField(m map[string]any, key string, loc *time.Location) time.Time {
	v, ok := find(m, key)
	if !ok {
		return time.Time{}
	}
	f, ok := toFloat(v)
	if !ok || f <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(f)).In(loc)
}

func decodeMap(raw json.RawMessage) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func decodeArray(raw json.RawMessage) ([]map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var rows []map[string]any
	if err := dec.Decode(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func find(m map[string]any, key string) (any, bool) {
	if v, ok := lookup(m, key); ok {
		return v, true
	}
	for _, v := range m {
		switch child := v.(type) {
		case map[string]any:
			if got, ok := find(child, key); ok {
				return got, true
			}
		case []any:
			for _, item := range child {
				if obj, ok := item.(map[string]any); ok {
					if got, ok := find(obj, key); ok {
						return got, true
					}
				}
			}
		}
	}
	return nil, false
}

func lookup(m map[string]any, key string) (any, bool) {
	for k, v := range m {
		if strings.EqualFold(k, key) {
			return v, true
		}
	}
	return nil, false
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case string:
		s := strings.TrimSpace(x)
		if s == "" || s == "--" || strings.EqualFold(s, "null") {
			return 0, false
		}
		f, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)
		return f, err == nil
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func textField(m map[string]any, key string) string {
	v, ok := find(m, key)
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	}
	return ""
}

func firstText(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := textField(m, k); v != "" {
			return v
		}
	}
	return ""
}

func numberField(m map[string]any, keys ...string) *float64 {
	for _, key := range keys {
		if v, ok := find(m, key); ok {
			if f, ok := toFloat(v); ok {
				return provider.Float(f)
			}
		}
	}
	return nil
}

func powerField(m map[string]any, candidates ...candidate) *float64 {
	for _, c := range candidates {
		v, ok := find(m, c.name)
		if !ok {
			continue
		}
		f, ok := toFloat(v)
		if !ok {
			continue
		}
		unit := textField(m, c.name+"Str")
		if unit == "" {
			unit = c.defaultUnit
		}
		return provider.Float(scalePower(f, unit))
	}
	return nil
}

func energyField(m map[string]any, candidates ...candidate) *float64 {
	for _, c := range candidates {
		v, ok := find(m, c.name)
		if !ok {
			continue
		}
		f, ok := toFloat(v)
		if !ok {
			continue
		}
		unit := textField(m, c.name+"Str")
		if unit == "" {
			unit = c.defaultUnit
		}
		return provider.Float(scaleEnergy(f, unit, c.defaultUnit))
	}
	return nil
}

func scalePower(v float64, unit string) float64 {
	u := strings.ToLower(strings.TrimSpace(unit))
	u = strings.TrimSuffix(u, "ac")
	u = strings.TrimSuffix(u, "dc")
	switch u {
	case "kw":
		return v * 1000
	case "mw":
		return v * 1e6
	case "gw":
		return v * 1e9
	case "", "w":
		return v
	default:
		return v
	}
}

func scaleEnergy(v float64, unit, fallback string) float64 {
	u := strings.ToLower(strings.TrimSpace(unit))
	switch u {
	case "mwh":
		return v * 1000
	case "wh":
		return v / 1000
	case "gwh":
		return v * 1e6
	case "kwh":
		return v
	case "":
		if strings.EqualFold(fallback, "MWh") {
			return v * 1000
		}
		return v
	default:
		return v
	}
}

// ValidateRawDetail reports whether raw looks like an inverter detail object.
func ValidateRawDetail(raw json.RawMessage) error {
	if len(raw) == 0 {
		return errors.New("solis: empty payload")
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return fmt.Errorf("solis: payload is not a JSON object: %w", err)
	}
	if len(m) == 0 {
		return errors.New("solis: payload is empty")
	}
	return nil
}
