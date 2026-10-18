package ui

import (
	"math"
	"strings"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func (m *Model) renderBattery(w, h, y int) string {
	var lines []string
	title := m.th.Bold(m.th.Accent).Render("BATTERY") + "  " + m.th.Muted.Render(m.snap.Model)
	lines = append(lines, title)

	soc := "—"
	direction := m.th.Muted.Render("no battery data")
	if m.snap.SOC != nil {
		soc = fmtPct(m.snap.SOC)
	}
	switch {
	case m.snap.BatteryW == nil:
	case *m.snap.BatteryW > 1:
		direction = m.th.Style(m.th.Battery).Render(fmtPowerAbs(*m.snap.BatteryW) + " charging")
	case *m.snap.BatteryW < -1:
		direction = m.th.Style(m.th.Discharge).Render(fmtPowerAbs(*m.snap.BatteryW) + " discharging")
	default:
		direction = m.th.Muted.Render("idle")
	}
	lines = append(lines, m.th.Bold(m.th.Battery).Render(soc)+"  "+direction+"   "+m.th.Muted.Render("SOH "+fmtPct(m.snap.SOH)))

	gauge := m.bigGauge(w)
	lines = append(lines, gauge...)

	rows := [][2]string{
		{"power", fmtPower(m.snap.BatteryW)},
		{"voltage", fmtVolts(m.snap.BatteryV)},
		{"current", fmtAmps(m.snap.BatteryA)},
		{"temperature", fmtTemp(m.snap.TempC)},
		{"reserve SOC", fmtPct(m.snap.ReserveSOC)},
		{"charge limit", fmtPct(m.snap.ChargeSOC)},
	}
	for i := 0; i < len(rows); i += 2 {
		left := kv(rows[i][0], rows[i][1], 14, m.th)
		right := ""
		if i+1 < len(rows) {
			right = kv(rows[i+1][0], rows[i+1][1], 14, m.th)
		}
		lines = append(lines, padRight(left, w/2)+right)
	}

	today := kv("today charged", m.th.Style(m.th.Battery).Render(fmtEnergy(m.snap.ChargeTodayKWh)), 14, m.th) +
		"   " + kv("discharged", m.th.Style(m.th.Discharge).Render(fmtEnergy(m.snap.DischargeTodayKWh)), 12, m.th)
	lines = append(lines, today)
	lifetime := kv("lifetime charged", m.th.Style(m.th.Battery).Render(fmtEnergy(m.snap.ChargeTotalKWh)), 17, m.th) +
		"   " + kv("discharged", m.th.Style(m.th.Discharge).Render(fmtEnergy(m.snap.DischargeTotalKWh)), 12, m.th)
	lines = append(lines, lifetime)

	if len(m.snap.Faults) > 0 {
		lines = append(lines, m.th.Bad.Render("faults: "+strings.Join(m.snap.Faults, " · ")))
	}

	chartH := h - len(lines) - 1
	if chartH >= 3 && len(m.daySamples) > 0 {
		lines = append(lines, m.th.Label.Render("24H SOC"))
		chartY := y + len(lines)
		chart := RenderLine(LineOptions{
			Width: w, Height: chartH, Series: m.socSeries(),
			Cursor: m.chartCursor, CursorColor: m.th.Fg, Dim: m.th.Faint,
			ShowYAxis: true, ShowXAxis: true, MinY: provider.Float(0), MaxY: provider.Float(100),
			XLabels: m.xLabels(),
			YLabel:  fmtPct1,
		})
		lines = append(lines, chart...)
		m.hits.Add(Rect{X: 0, Y: chartY, W: w, H: chartH}, Action{Kind: ActionChartCursor, Rect: Rect{X: 0, Y: chartY, W: w, H: chartH}})
	} else if len(m.daySamples) == 0 {
		lines = append(lines, m.th.Muted.Render("no SOC history yet"))
	}
	return strings.Join(fitLines(lines, w, h), "\n")
}

func (m *Model) bigGauge(w int) []string {
	cells := clamp(w-8, 20, 72)
	filled := 0
	if m.snap.SOC != nil {
		filled = clamp(int(math.Round(*m.snap.SOC/100*float64(cells))), 0, cells)
	}
	color := m.th.Battery
	if m.snap.SOC != nil {
		switch {
		case *m.snap.SOC < 20:
			color = m.th.Danger
		case *m.snap.SOC < 50:
			color = m.th.Warning
		default:
			color = m.th.Green
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
			bar.WriteString(m.th.Style(color).Render("█"))
		default:
			bar.WriteString(m.th.Style(color).Render("░"))
		}
	}
	return []string{bar.String()}
}

func (m *Model) socSeries() []LineSeries {
	n := len(m.daySamples)
	if n == 0 {
		return nil
	}
	vals := make([]*float64, n)
	for i, s := range m.daySamples {
		vals[i] = s.SOC
	}
	return []LineSeries{{Name: "soc", Color: m.th.Battery, Values: vals}}
}
