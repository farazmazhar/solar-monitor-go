// Package demo provides a deterministic, network-free implementation of
// provider.Source. It simulates a sunny day so the UI can be developed,
// tested and screenshotted without credentials.
package demo

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

const (
	stationName = "Home Rooftop"
	modelName   = "S6-EH1P6K-L-PRO"
	serial      = "DEMO-S6-0001"
	capacityKWh = 20.0
	maxChargeKW = 1.8
	maxDischKW  = 3.0
	reserveSOC  = 20.0
	startSOC    = 12.0
	currency    = "PKR"
	ratePKR     = 27.0
)

// Options control the simulation.
type Options struct {
	// Speed is simulated minutes per real second. Zero means 0.5, so five
	// simulated minutes pass every ten real seconds.
	Speed float64
	// Start is the synthetic wall clock at launch. Zero means today 13:20.
	Start time.Time
	// Clock overrides the synthetic clock, for deterministic tests.
	Clock func() time.Time
}

// Provider simulates one inverter.
type Provider struct {
	clock func() time.Time
}

// New returns a running simulation.
func New(opts Options) *Provider {
	if opts.Clock != nil {
		return &Provider{clock: opts.Clock}
	}
	speed := opts.Speed
	if speed <= 0 {
		speed = 0.5
	}
	origin := opts.Start
	if origin.IsZero() {
		now := time.Now()
		origin = time.Date(now.Year(), now.Month(), now.Day(), 13, 20, 0, 0, now.Location())
	}
	started := time.Now()
	return &Provider{clock: func() time.Time {
		elapsed := time.Since(started).Seconds() * speed
		return origin.Add(time.Duration(elapsed * float64(time.Minute)))
	}}
}

// Name implements provider.Source.
func (p *Provider) Name() string { return "demo" }

// Close implements provider.Source.
func (p *Provider) Close() error { return nil }

// Now returns the current synthetic time.
func (p *Provider) Now() time.Time { return p.clock() }

// Snapshot implements provider.Source.
func (p *Provider) Snapshot(context.Context) (provider.Snapshot, error) {
	now := p.Now()
	v := simulate(now)
	return provider.Snapshot{
		FetchedAt: now,
		DataAt:    now.Truncate(5 * time.Minute),
		Source:    "demo",
		Station:   stationName,
		Model:     modelName,
		SN:        serial,
		State:     provider.StateOnline,

		SolarW:      provider.Float(v.solarW),
		LoadW:       provider.Float(v.loadW),
		BackupW:     provider.Float(v.backupW),
		GridW:       provider.Float(v.gridW),
		BatteryW:    provider.Float(v.batteryW),
		GridImportW: provider.Float(v.importW),
		GridExportW: provider.Float(v.exportW),

		SOC:        provider.Float(v.soc),
		SOH:        provider.Float(97.4),
		TempC:      provider.Float(v.tempC),
		BatteryV:   provider.Float(v.batteryV),
		BatteryA:   provider.Float(v.batteryA),
		GridHz:     provider.Float(50.01),
		GridV:      provider.Float(231.4),
		ReserveSOC: provider.Float(reserveSOC),
		ChargeSOC:  provider.Float(98),

		PVStrings: v.strings,

		TodayKWh:          provider.Float(v.todayKWh),
		MonthKWh:          provider.Float(monthToDate(now)),
		YearKWh:           provider.Float(yearToDate(now)),
		TotalKWh:          provider.Float(8421.6 + yearToDate(now)),
		LoadTodayKWh:      provider.Float(v.loadKWh),
		ImportTodayKWh:    provider.Float(v.importKWh),
		ExportTodayKWh:    provider.Float(v.exportKWh),
		ChargeTodayKWh:    provider.Float(v.chargeKWh),
		DischargeTodayKWh: provider.Float(v.dischargeKWh),
		ChargeTotalKWh:    provider.Float(2481.9),
		DischargeTotalKWh: provider.Float(2217.4),

		IncomeDay:   provider.Float(v.todayKWh * ratePKR),
		IncomeMonth: provider.Float(monthToDate(now) * ratePKR),
		IncomeYear:  provider.Float(yearToDate(now) * ratePKR),
		IncomeTotal: provider.Float((8421.6 + yearToDate(now)) * ratePKR),
		Currency:    currency,

		Flow: v.flow,

		Raw: demoRaw(now, v),
	}, nil
}

// demoRaw builds a small SolisCloud-shaped payload so the diagnostics browser
// has something realistic to display in demo mode.
func demoRaw(now time.Time, v simValues) json.RawMessage {
	m := map[string]any{
		"stationName":           stationName,
		"sn":                    serial,
		"model":                 modelName,
		"machine":               modelName,
		"state":                 1,
		"currentState":          "3",
		"faultCodeDesc":         "Generating",
		"timeStr":               now.Format("2006-01-02 15:04:05"),
		"inverterTemperature":   round1(31 + 10*v.solarW/5600),
		"pac":                   round2(v.solarW / 1000),
		"pacStr":                "kW",
		"psum":                  round2(v.gridW / 1000),
		"psumStr":               "kW",
		"familyLoadPower":       round2(v.loadW / 1000),
		"familyLoadPowerStr":    "kW",
		"backupPowerA":          round0(v.backupW),
		"batteryPower":          round2(v.batteryW / 1000),
		"batteryPowerStr":       "kW",
		"batteryCapacitySoc":    round1(v.soc),
		"batteryHealthSoh":      97.4,
		"eToday":                round2(v.todayKWh),
		"eTodayStr":             "kWh",
		"eMonth":                round2(monthToDate(now)),
		"eYear":                 round1(yearToDate(now) / 1000),
		"eYearStr":              "MWh",
		"allEnergyOriginal":     round1(8421.6 + yearToDate(now)),
		"storageBatteryVoltage": round1(v.batteryV),
		"storageBatteryCurrent": round1(v.batteryA),
		"dcInputType":           1,
		"uPv1":                  356.0,
		"iPv1":                  round1(v.solarW * 0.62 / 356),
		"uPv2":                  349.0,
		"iPv2":                  round1(v.solarW * 0.38 / 349),
		"fac":                   50.01,
		"uAc1":                  231.4,
	}
	b, _ := json.Marshal(m)
	return b
}

func round0(v float64) float64 { return math.Round(v) }
func round1(v float64) float64 { return math.Round(v*10) / 10 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

// Today implements provider.Source.
func (p *Provider) Today(_ context.Context, day time.Time) ([]provider.Sample, error) {
	now := p.Now()
	loc := now.Location()
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	if start.After(now) {
		return nil, nil
	}
	end := start.Add(24 * time.Hour)
	if sameDay(day, now) {
		end = now
	}

	var out []provider.Sample
	for t := start; !t.After(end); t = t.Add(5 * time.Minute) {
		v := simulateAt(start, t)
		out = append(out, provider.Sample{
			TS:       t,
			SolarW:   provider.Float(v.solarW),
			LoadW:    provider.Float(v.loadW),
			GridW:    provider.Float(v.gridW),
			BatteryW: provider.Float(v.batteryW),
			SOC:      provider.Float(v.soc),
			ImportW:  provider.Float(v.importW),
			ExportW:  provider.Float(v.exportW),
		})
	}
	return out, nil
}

// Month implements provider.Source.
func (p *Provider) Month(_ context.Context, year int, month time.Month) ([]provider.DayEnergy, error) {
	now := p.Now()
	loc := now.Location()
	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	next := first.AddDate(0, 1, 0)
	last := next.AddDate(0, 0, -1)
	if last.After(now) {
		last = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	}
	if first.After(now) {
		return nil, nil
	}

	var out []provider.DayEnergy
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		gen, cons, imp, exp, chg, dis := dayEnergy(d)
		out = append(out, provider.DayEnergy{
			Date:          d,
			GeneratedKWh:  provider.Float(gen),
			ConsumedKWh:   provider.Float(cons),
			ImportedKWh:   provider.Float(imp),
			ExportedKWh:   provider.Float(exp),
			ChargedKWh:    provider.Float(chg),
			DischargedKWh: provider.Float(dis),
			Income:        provider.Float(gen * ratePKR),
			Currency:      currency,
		})
	}
	return out, nil
}

// Year implements provider.Source.
func (p *Provider) Year(_ context.Context, year int) ([]provider.MonthEnergy, error) {
	now := p.Now()
	loc := now.Location()
	lastMonth := time.Month(12)
	if year == now.Year() {
		lastMonth = now.Month()
	}
	if year > now.Year() {
		return nil, nil
	}
	var out []provider.MonthEnergy
	for m := time.January; m <= lastMonth; m++ {
		days := time.Date(year, m+1, 1, 0, 0, 0, 0, loc).AddDate(0, 0, -1).Day()
		if m == time.February && isLeap(year) {
			days = 29
		}
		var gen, cons, imp, exp, chg, dis float64
		for d := 1; d <= days; d++ {
			g, c, i, e, ch, di := dayEnergy(time.Date(year, m, d, 0, 0, 0, 0, loc))
			gen += g
			cons += c
			imp += i
			exp += e
			chg += ch
			dis += di
		}
		out = append(out, provider.MonthEnergy{
			Month:         time.Date(year, m, 1, 0, 0, 0, 0, loc),
			GeneratedKWh:  provider.Float(gen),
			ConsumedKWh:   provider.Float(cons),
			ImportedKWh:   provider.Float(imp),
			ExportedKWh:   provider.Float(exp),
			ChargedKWh:    provider.Float(chg),
			DischargedKWh: provider.Float(dis),
			Income:        provider.Float(gen * ratePKR),
			Currency:      currency,
		})
	}
	return out, nil
}

// YearDays implements provider.YearDaysSource for the heatmap.
func (p *Provider) YearDays(ctx context.Context, year int) ([]provider.DayEnergy, error) {
	var out []provider.DayEnergy
	for mo := time.January; mo <= time.December; mo++ {
		rows, err := p.Month(ctx, year, mo)
		if err != nil {
			return out, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

// Alarms implements provider.Source.
func (p *Provider) Alarms(_ context.Context, from, to time.Time) ([]provider.Alarm, error) {
	now := p.Now()
	loc := now.Location()
	all := []provider.Alarm{
		{
			ID:      "demo-1",
			Level:   2,
			Code:    "1030",
			Message: "Grid voltage high",
			Advice:  "The inverter resumed automatically. Check grid voltage if this repeats.",
			State:   "restored",
			Begin:   now.AddDate(0, 0, -2).Add(-4 * time.Hour),
			End:     now.AddDate(0, 0, -2).Add(-3*time.Hour - 42*time.Minute),
		},
		{
			ID:      "demo-2",
			Level:   1,
			Code:    "2011",
			Message: "Battery communication warning",
			Advice:  "Usually transient. Verify the BMS cable if it persists.",
			State:   "restored",
			Begin:   now.AddDate(0, 0, -8).Add(-9 * time.Hour),
			End:     now.AddDate(0, 0, -8).Add(-8*time.Hour - 51*time.Minute),
		},
		{
			ID:      "demo-3",
			Level:   1,
			Code:    "1055",
			Message: "Fan speed derating",
			Advice:  "Clean the air path and check that the inverter is not enclosed.",
			State:   "restored",
			Begin:   now.AddDate(0, 0, -17).Add(-2 * time.Hour),
			End:     now.AddDate(0, 0, -17).Add(-time.Hour - 20*time.Minute),
		},
	}
	var out []provider.Alarm
	for _, a := range all {
		if a.Begin.Before(from) || a.Begin.After(to) {
			continue
		}
		if a.Begin.Location() != loc {
			a.Begin = a.Begin.In(loc)
			a.End = a.End.In(loc)
		}
		out = append(out, a)
	}
	return out, nil
}

type simValues struct {
	solarW, loadW, backupW, gridW, batteryW float64
	importW, exportW                        float64
	soc, tempC, batteryV, batteryA          float64
	todayKWh, loadKWh, importKWh, exportKWh float64
	chargeKWh, dischargeKWh                 float64
	strings                                 []provider.PVString
	flow                                    provider.Flow
}

// simulate runs the model from local midnight to now.
func simulate(now time.Time) simValues {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return simulateAt(start, now)
}

// simulateAt runs the model from start to now. start is local midnight.
func simulateAt(start, now time.Time) simValues {
	soc := startSOC
	var v simValues
	const dt = 5 * time.Minute
	elapsedHours := 0.0

	for t := start; !t.After(now); t = t.Add(dt) {
		solar := solarKW(t) * 1000
		load := loadKW(t) * 1000
		backup := backupKW(t) * 1000

		surplus := solar - load
		var charge, discharge, export, importW float64
		if surplus >= 0 {
			room := math.Max(0, (98-soc)/100*capacityKWh)
			charge = math.Min(surplus, math.Min(maxChargeKW*1000, room*1000/0.0833))
			export = surplus - charge
		} else {
			avail := math.Max(0, (soc-reserveSOC)/100*capacityKWh)
			discharge = math.Min(-surplus, math.Min(maxDischKW*1000, avail*1000/0.0833))
			importW = -surplus - discharge
		}

		soc += (charge - discharge) * 0.0833 / 1000 / capacityKWh * 100
		soc = math.Max(0, math.Min(100, soc))
		elapsedHours += dt.Hours()

		hours := dt.Hours()
		v.todayKWh += solar / 1000 * hours
		v.loadKWh += load / 1000 * hours
		v.importKWh += importW / 1000 * hours
		v.exportKWh += export / 1000 * hours
		v.chargeKWh += charge / 1000 * hours
		v.dischargeKWh += discharge / 1000 * hours

		if t.Equal(now.Truncate(dt)) || now.Sub(t) < dt {
			v.solarW = solar
			v.loadW = load
			v.backupW = backup
			v.gridW = export - importW
			v.batteryW = charge - discharge
			v.importW = importW
			v.exportW = export
			v.soc = soc
			v.tempC = 31 + 10*solar/5600
			v.batteryV = 49.2 + (soc-50)/100*5.6
			if v.batteryV < 44 {
				v.batteryV = 44
			}
			if charge > 0 {
				v.batteryA = charge / v.batteryV
			} else {
				v.batteryA = -discharge / v.batteryV
			}
			if math.Abs(v.batteryA) < 0.05 {
				v.batteryA = 0
			}
			v.flow = flowFor(solar, load, charge, discharge, export, importW)
		}
	}

	v.strings = pvStrings(v.solarW, now)
	return v
}

func solarKW(t time.Time) float64 {
	h := float64(t.Hour()) + float64(t.Minute())/60
	sunrise, sunset := 6.15, 18.55
	if h <= sunrise || h >= sunset {
		return 0
	}
	x := (h - sunrise) / (sunset - sunrise)
	clear := math.Pow(math.Sin(math.Pi*x), 1.35)
	cloud := 1 - 0.06*math.Sin(h*2.7) - 0.03*math.Sin(h*7.1)
	return math.Max(0, 5.65*clear*cloud)
}

func loadKW(t time.Time) float64 {
	h := float64(t.Hour()) + float64(t.Minute())/60
	base := 0.42
	morning := 0.55 * math.Exp(-math.Pow((h-7.0)/1.6, 2))
	evening := 1.15 * math.Exp(-math.Pow((h-19.0)/2.1, 2))
	midday := 0.25 * math.Exp(-math.Pow((h-13.0)/2.6, 2))
	ripple := 0.08 * math.Sin(h*11.3)
	return math.Max(0.28, base+morning+evening+midday+ripple)
}

func backupKW(t time.Time) float64 {
	h := float64(t.Hour()) + float64(t.Minute())/60
	if h < 6 || h > 23 {
		return 0.22
	}
	return 0.27 + 0.03*math.Sin(h*3.1)
}

func pvStrings(solarW float64, now time.Time) []provider.PVString {
	solarW = math.Max(0, solarW)
	split := 0.62
	if now.Hour() >= 15 {
		split = 0.48
	}
	a := solarW * split
	b := solarW - a
	mk := func(i int, w float64, volts float64) provider.PVString {
		if w <= 0 {
			return provider.PVString{Index: i, Voltage: provider.Float(volts), Current: provider.Float(0), Power: provider.Float(0)}
		}
		return provider.PVString{
			Index:   i,
			Voltage: provider.Float(volts),
			Current: provider.Float(w / volts),
			Power:   provider.Float(w),
		}
	}
	return []provider.PVString{mk(1, a, 356), mk(2, b, 349)}
}

func flowFor(solar, load, charge, discharge, export, importW float64) provider.Flow {
	f := provider.Flow{Balanced: true}
	if solar >= load {
		f.PVToHouse = load
		f.PVToBattery = charge
		f.PVToGrid = export
	} else {
		f.PVToHouse = solar
		if discharge > 0 {
			f.BatteryToHouse = math.Min(discharge, load-solar)
		}
		f.GridToHouse = math.Min(importW, math.Max(0, load-solar-f.BatteryToHouse))
		if charge > 0 {
			f.GridToBattery = charge
		}
	}
	return f
}

func dayEnergy(d time.Time) (gen, cons, imp, exp, chg, dis float64) {
	r := rand.New(rand.NewSource(int64(d.Year()*1000 + d.YearDay())))
	season := 1 - 0.18*math.Cos(2*math.Pi*float64(d.YearDay()-172)/365)
	gen = (19 + r.Float64()*11) * season
	cons = 12 + r.Float64()*8
	imp = math.Max(0, cons-gen*0.72)
	exp = math.Max(0, gen*0.72-cons)
	chg = math.Min(gen*0.35, 9)
	dis = math.Min(cons*0.45, 8)
	return
}

func monthToDate(now time.Time) float64 {
	var total float64
	for d := 1; d <= now.Day(); d++ {
		g, _, _, _, _, _ := dayEnergy(time.Date(now.Year(), now.Month(), d, 0, 0, 0, 0, now.Location()))
		total += g
	}
	return total
}

func yearToDate(now time.Time) float64 {
	var total float64
	for m := time.January; m <= now.Month(); m++ {
		days := 30
		if m == now.Month() {
			days = now.Day()
		}
		for d := 1; d <= days; d++ {
			g, _, _, _, _, _ := dayEnergy(time.Date(now.Year(), m, d, 0, 0, 0, 0, now.Location()))
			total += g
		}
	}
	return total
}

func sameDay(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}

func isLeap(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}
