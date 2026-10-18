package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

type tile struct {
	label string
	value string
	sub   string
	color color.Color
	kind  Tile
	spark []*float64
}

func (m *Model) tileData() []tile {
	gridColor := m.th.Grid
	if m.snap.GridW != nil {
		if *m.snap.GridW > 1 {
			gridColor = m.th.Export
		} else if *m.snap.GridW < -1 {
			gridColor = m.th.Import
		}
	}
	batColor := m.th.Battery
	batSub := "—"
	if m.snap.BatteryW != nil {
		if *m.snap.BatteryW > 1 {
			batSub = fmtPowerAbs(*m.snap.BatteryW) + " charging"
		} else if *m.snap.BatteryW < -1 {
			batSub = fmtPowerAbs(*m.snap.BatteryW) + " discharging"
			batColor = m.th.Discharge
		}
	}
	gridValue := fmtPowerAbs(provider.Conv(m.snap.GridW))
	gridDir := ""
	switch {
	case m.snap.GridW == nil:
	case *m.snap.GridW > 1:
		gridDir = " EXP"
	case *m.snap.GridW < -1:
		gridDir = " IMP"
	}
	n := 4
	if m.width >= 100 {
		n = 6
	}
	compact := n == 6
	solarSub := "today " + fmtEnergy(m.snap.TodayKWh)
	houseSub := "bkp " + fmtPower(m.snap.BackupW)
	gridSub := "in " + shortEnergy(m.snap.ImportTodayKWh) + " · out " + shortEnergy(m.snap.ExportTodayKWh)
	batSub2 := batSub
	if compact {
		solarSub = "today " + shortEnergy(m.snap.TodayKWh) + " kWh"
		gridSub = gridWord(m.snap)
		batSub2 = batWord(m.snap)
	}
	tiles := []tile{
		{
			label: "SOLAR", value: fmtPower(m.snap.SolarW),
			sub: solarSub, color: m.th.Solar, kind: TileSolar, spark: m.spark(TileSolar),
		},
		{
			label: "HOUSE", value: fmtPower(m.snap.LoadW),
			sub: houseSub, color: m.th.Green, kind: TileHouse, spark: m.spark(TileHouse),
		},
		{
			label: "GRID", value: gridValue + gridDir,
			sub:   gridSub,
			color: gridColor, kind: TileGrid, spark: m.spark(TileGrid),
		},
		{
			label: "BATTERY", value: fmtPct(m.snap.SOC),
			sub: batSub2, color: batColor, kind: TileBattery, spark: m.spark(TileBattery),
		},
	}
	if m.width >= 100 {
		tiles = append(tiles,
			tile{
				label: "INCOME", value: fmtMoney(m.currencySymbol(), m.incomeToday()),
				sub: "today", color: m.th.Green, kind: TileIncome, spark: m.spark(TileSolar),
			},
			tile{
				label: "CO₂", value: fmtCO2(m.co2Today()),
				sub: "avoided today", color: m.th.Green, kind: TileCO2, spark: m.spark(TileSolar),
			},
		)
	}
	return tiles
}

func (m *Model) renderGauge(w int) []string {
	pct := "—"
	filled := 0
	cells := clamp(w-46, 12, 44)
	if m.snap.SOC != nil {
		pct = fmtPct(m.snap.SOC)
		filled = clamp(int(math.Round(*m.snap.SOC/100*float64(cells))), 0, cells)
	}
	gaugeColor := m.th.Battery
	if m.snap.SOC != nil {
		switch {
		case *m.snap.SOC < 20:
			gaugeColor = m.th.Danger
		case *m.snap.SOC < 50:
			gaugeColor = m.th.Warning
		default:
			gaugeColor = m.th.Green
		}
	}
	reserve := -1
	if m.snap.ReserveSOC != nil {
		reserve = clamp(int(math.Round(*m.snap.ReserveSOC/100*float64(cells))), 0, cells-1)
	}
	var bar strings.Builder
	for i := 0; i < cells; i++ {
		switch {
		case i == reserve:
			bar.WriteString(m.th.Muted.Render("│"))
		case i < filled:
			bar.WriteString(m.th.Style(gaugeColor).Render("█"))
		default:
			bar.WriteString(m.th.Style(gaugeColor).Render("░"))
		}
	}
	direction := ""
	if m.snap.BatteryW != nil {
		switch {
		case *m.snap.BatteryW > 1:
			direction = m.th.Style(m.th.Battery).Render(fmtPowerAbs(*m.snap.BatteryW) + " charging")
		case *m.snap.BatteryW < -1:
			direction = m.th.Style(m.th.Discharge).Render(fmtPowerAbs(*m.snap.BatteryW) + " discharging")
		default:
			direction = m.th.Muted.Render("idle")
		}
	}
	line1 := m.th.Label.Render("BATTERY ") + bar.String() + " " + m.th.Bold(gaugeColor).Render(pct) + "  " + direction

	var detail []string
	if m.snap.SOH != nil {
		detail = append(detail, "SOH "+fmtPct(m.snap.SOH))
	}
	if m.snap.BatteryV != nil {
		detail = append(detail, fmtVolts(m.snap.BatteryV))
	}
	if m.snap.BatteryA != nil {
		detail = append(detail, fmtAmps(m.snap.BatteryA))
	}
	if m.snap.ReserveSOC != nil {
		detail = append(detail, "reserve "+fmtPct(m.snap.ReserveSOC))
	}
	if m.snap.ChargeSOC != nil {
		detail = append(detail, "charge to "+fmtPct(m.snap.ChargeSOC))
	}
	line2 := m.th.Muted.Render("  " + strings.Join(detail, " · "))
	return []string{line1, line2}
}

func (m *Model) currencySymbol() string {
	if m.snap.Currency != "" && strings.EqualFold(m.snap.Currency, m.cfg.Display.Currency) {
		if m.cfg.Display.CurrencySymbol != "" {
			return m.cfg.Display.CurrencySymbol
		}
		return m.snap.Currency
	}
	if m.cfg.Display.CurrencySymbol != "" {
		return m.cfg.Display.CurrencySymbol
	}
	return m.cfg.Display.Currency
}

// incomeToday returns API income when its currency matches, else a tariff
// estimate, else nil so the UI can say "—".
func (m *Model) incomeToday() *float64 {
	if m.snap.IncomeDay != nil && m.snap.Currency != "" && strings.EqualFold(m.snap.Currency, m.cfg.Display.Currency) {
		return m.snap.IncomeDay
	}
	if m.cfg.Tariff.Import == 0 && m.cfg.Tariff.Export == 0 {
		return nil
	}
	gen := provider.Conv(m.snap.TodayKWh)
	exp := provider.Conv(m.snap.ExportTodayKWh)
	value := exp*m.cfg.Tariff.Export + math.Max(0, gen-exp)*m.cfg.Tariff.Import
	return provider.Float(value)
}

func (m *Model) co2Today() *float64 {
	if m.snap.TodayKWh == nil || m.cfg.Display.CO2KgPerKWh == 0 {
		return nil
	}
	return provider.Float(*m.snap.TodayKWh * m.cfg.Display.CO2KgPerKWh)
}

func (m *Model) spark(kind Tile) []*float64 {
	out := make([]*float64, 0, len(m.daySamples))
	for _, s := range m.daySamples {
		var v *float64
		switch kind {
		case TileSolar:
			v = s.SolarW
		case TileHouse:
			v = s.LoadW
		case TileGrid:
			if s.GridW != nil {
				v = provider.Float(math.Abs(*s.GridW))
			}
		case TileBattery:
			if s.BatteryW != nil {
				v = provider.Float(math.Abs(*s.BatteryW))
			}
		default:
			v = s.SolarW
		}
		out = append(out, v)
	}
	return out
}

func styleFg(th Theme, c color.Color, s string) string {
	return th.Style(c).Render(s)
}

func kv(label, value string, labelW int, th Theme) string {
	return th.Label.Render(padRight(label, labelW)) + value
}

func (m *Model) headerStats(w int) string {
	parts := []string{
		m.th.Muted.Render("today ") + m.th.Style(m.th.Solar).Render(shortEnergy(m.snap.TodayKWh)),
		m.th.Muted.Render("load ") + m.th.Style(m.th.Fg).Render(shortEnergy(m.snap.LoadTodayKWh)),
		m.th.Muted.Render("imp ") + m.th.Style(m.th.Import).Render(shortEnergy(m.snap.ImportTodayKWh)),
		m.th.Muted.Render("exp ") + m.th.Style(m.th.Export).Render(shortEnergy(m.snap.ExportTodayKWh)),
		m.th.Muted.Render("income ") + m.th.Good.Render(fmtMoney(m.currencySymbol(), m.incomeToday())),
		m.th.Muted.Render("CO₂ ") + m.th.Good.Render(fmtCO2(m.co2Today())),
	}
	return clampText(strings.Join(parts, "   "), w)
}

func gridWord(s provider.Snapshot) string {
	if s.GridW == nil {
		return "—"
	}
	switch {
	case *s.GridW > 1:
		return "exporting"
	case *s.GridW < -1:
		return "importing"
	default:
		return "idle"
	}
}

func batWord(s provider.Snapshot) string {
	if s.BatteryW == nil {
		return "—"
	}
	switch {
	case *s.BatteryW > 1:
		return "charging"
	case *s.BatteryW < -1:
		return "discharging"
	default:
		return "idle"
	}
}

func fmtCO2(kg *float64) string {
	if kg == nil {
		return "—"
	}
	if *kg >= 1000 {
		return fmt.Sprintf("%.2f t", *kg/1000)
	}
	return fmt.Sprintf("%.1f kg", *kg)
}

func humanBytes(n int64) string {
	switch {
	case n <= 0:
		return "—"
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	case n < 1024*1024:
		return fmt.Sprintf("%.1f KiB", float64(n)/1024)
	case n < 1024*1024*1024:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1024*1024))
	default:
		return fmt.Sprintf("%.2f GiB", float64(n)/(1024*1024*1024))
	}
}
