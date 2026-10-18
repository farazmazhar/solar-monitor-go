package ui

import (
	"image/color"
	"strings"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func (m *Model) renderToday(w, h, y int) string {
	peak, peakAt := dayPeak(m.daySamples)
	title := m.th.Bold(m.th.Accent).Render("TODAY") + "  " + m.th.Muted.Render(fmtDate(m.day))
	if peak > 0 {
		title = joinSides(title, m.th.Muted.Render("peak ")+m.th.Style(m.th.Solar).Render(fmtPowerAbs(peak))+m.th.Muted.Render(" at "+peakAt.Format("15:04")), w)
	}

	summary := []string{
		"  " + m.th.Muted.Render("generated ") + m.th.Style(m.th.Solar).Render(fmtEnergy(m.snap.TodayKWh)) +
			m.th.Muted.Render("     consumed ") + m.th.Value.Render(fmtEnergy(m.snap.LoadTodayKWh)),
		"  " + m.th.Muted.Render("imported ") + m.th.Style(m.th.Import).Render(fmtEnergy(m.snap.ImportTodayKWh)) +
			m.th.Muted.Render("     exported ") + m.th.Style(m.th.Export).Render(fmtEnergy(m.snap.ExportTodayKWh)) +
			m.th.Muted.Render("     battery +") + m.th.Style(m.th.Battery).Render(fmtEnergy(m.snap.ChargeTodayKWh)) +
			m.th.Muted.Render(" / -") + m.th.Style(m.th.Discharge).Render(fmtEnergy(m.snap.DischargeTodayKWh)),
		"  " + m.th.Muted.Render("income ") + m.th.Good.Render(fmtMoney(m.currencySymbol(), m.incomeToday())) +
			m.th.Muted.Render("     CO₂ ") + m.th.Good.Render(fmtCO2(m.co2Today())) +
			m.th.Muted.Render("     temperature ") + m.th.Value.Render(fmtTempC(m.snap.TempC)),
	}

	var lines []string
	lines = append(lines, title, "")
	lines = append(lines, wrapPanel("SUMMARY", summary, w, m.th.Faint, m.th.Dim, m.th.Faint)...)
	lines = append(lines, "")
	lines = append(lines, m.dayLegend(w))
	lines = append(lines, m.cursorLine(w))

	chartH := h - len(lines)
	if chartH < 6 {
		chartH = 6
	}
	chartY := y + len(lines)
	chart, hit := m.renderDayChart(w, chartH, "POWER · FIXED DAY 00:00-23:59", m.daySamples, m.day, m.series, true, true)
	lines = append(lines, chart...)
	m.hits.Add(
		Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H},
		Action{Kind: ActionChartCursor, Rect: Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H}},
	)
	return strings.Join(fitLines(lines, w, h), "\n")
}

func (m *Model) cursorLine(w int) string {
	s := m.selectedSample()
	if s == nil {
		return m.th.Muted.Render("waiting for today's curve")
	}
	parts := []string{
		m.th.Bold(m.th.Fg).Render(s.TS.Format("15:04")),
		styleFg(m.th, m.th.Solar, "solar ") + m.th.Muted.Render(fmtPower(s.SolarW)),
		styleFg(m.th, m.th.Fg, "consumption ") + m.th.Muted.Render(fmtPower(s.LoadW)),
		styleFg(m.th, m.th.Grid, "grid ") + m.th.Muted.Render(fmtPower(s.GridW)),
		styleFg(m.th, m.th.Battery, "battery ") + m.th.Muted.Render(fmtPower(s.BatteryW)),
	}
	return clampText(strings.Join(parts, "   "), w)
}

// dayLegend is the series key and toggle hint above the Today chart.
func (m *Model) dayLegend(w int) string {
	if m.chartView == ViewMix {
		return m.mixLegend(w)
	}
	entries := []struct {
		key   string
		name  string
		color color.Color
		on    bool
	}{
		{"s", "solar", m.th.Solar, m.series[0]},
		{"l", "consumption", m.th.Fg, m.series[1]},
		{"g", "grid", m.th.Grid, m.series[2]},
		{"b", "battery", m.th.Battery, m.series[3]},
	}
	var parts []string
	for _, e := range entries {
		label := "[" + e.key + "] "
		if e.on {
			parts = append(parts, m.th.HelpKey.Render(label)+styleFg(m.th, e.color, e.name))
		} else {
			parts = append(parts, m.th.Muted.Render(label+e.name))
		}
	}
	return clampText(strings.Join(parts, "   "), w)
}

func dayPeak(samples []provider.Sample) (float64, time.Time) {
	peak := 0.0
	var at time.Time
	for _, s := range samples {
		if s.SolarW != nil && *s.SolarW > peak {
			peak = *s.SolarW
			at = s.TS
		}
	}
	return peak, at
}
