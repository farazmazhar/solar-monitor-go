package ui

import (
	"image/color"
	"strings"
	"time"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

func (m *Model) renderToday(w, h, y int) string {
	var lines []string
	title := m.th.Bold(m.th.Accent).Render("TODAY") + "  " + m.th.Muted.Render(fmtDate(m.day))
	peak, peakAt := dayPeak(m.daySamples)
	if peak > 0 {
		title += m.th.Muted.Render("   peak ") + m.th.Style(m.th.Solar).Render(fmtPowerAbs(peak)) +
			m.th.Muted.Render(" at "+peakAt.Format("15:04"))
	}
	lines = append(lines, title)
	lines = append(lines, m.headerStats(w))
	lines = append(lines, m.seriesLegend(w))
	lines = append(lines, m.cursorLine(w))

	if len(m.daySamples) == 0 {
		lines = append(lines, centerText(m.th.Muted.Render("no samples for this day yet"), w))
		return strings.Join(fitLines(lines, w, h), "\n")
	}

	chartH := h - len(lines)
	if chartH < 3 {
		chartH = 3
	}
	chartY := y + len(lines)
	chart := RenderLine(LineOptions{
		Width: w, Height: chartH,
		Series: m.todaySeries(),
		Cursor: m.chartCursor, CursorColor: m.th.Fg, Dim: m.th.Faint,
		ShowYAxis: true, ShowXAxis: true, ZeroLine: true,
		XLabels: m.xLabels(),
	})
	lines = append(lines, chart...)
	m.hits.Add(Rect{X: 0, Y: chartY, W: w, H: chartH}, Action{Kind: ActionChartCursor, Rect: Rect{X: 0, Y: chartY, W: w, H: chartH}})
	return strings.Join(fitLines(lines, w, h), "\n")
}

func (m *Model) seriesLegend(w int) string {
	current := provider.Sample{}
	if len(m.daySamples) > 0 {
		current = m.daySamples[len(m.daySamples)-1]
	}
	if m.chartCursor >= 0 && m.chartCursor < len(m.daySamples) {
		current = m.daySamples[m.chartCursor]
	}
	entries := []struct {
		name  string
		value string
		c     color.Color
		on    bool
	}{
		{"s", "solar " + fmtPower(current.SolarW), m.th.Solar, m.series[0]},
		{"l", "load " + fmtPower(current.LoadW), m.th.Fg, m.series[1]},
		{"g", "grid " + fmtPower(current.GridW), m.th.Grid, m.series[2]},
		{"b", "battery " + fmtPower(current.BatteryW), m.th.Battery, m.series[3]},
	}
	var parts []string
	for _, e := range entries {
		label := m.th.HelpKey.Render(e.name) + " "
		if e.on {
			parts = append(parts, label+m.th.Style(e.c).Render(e.value))
		} else {
			parts = append(parts, label+m.th.Muted.Render(e.value))
		}
	}
	return clampText(strings.Join(parts, "   "), w)
}

func (m *Model) cursorLine(w int) string {
	if len(m.daySamples) == 0 {
		return m.th.Muted.Render("waiting for today's curve")
	}
	s := m.daySamples[len(m.daySamples)-1]
	if m.chartCursor >= 0 && m.chartCursor < len(m.daySamples) {
		s = m.daySamples[m.chartCursor]
	}
	parts := []string{
		m.th.Bold(m.th.Fg).Render(s.TS.Format("15:04")),
		m.th.Style(m.th.Solar).Render("solar " + fmtPower(s.SolarW)),
		m.th.Style(m.th.Fg).Render("load " + fmtPower(s.LoadW)),
		m.th.Style(m.th.Grid).Render("grid " + fmtPower(s.GridW)),
		m.th.Style(m.th.Battery).Render("battery " + fmtPower(s.BatteryW)),
	}
	if s.SOC != nil {
		parts = append(parts, m.th.Style(m.th.Battery).Render("soc "+fmtPct(s.SOC)))
	}
	return clampText(strings.Join(parts, "  "), w)
}

func (m *Model) xLabels() []string {
	if len(m.daySamples) == 0 {
		return []string{"00:00", "12:00", "23:55"}
	}
	first := m.daySamples[0].TS
	last := m.daySamples[len(m.daySamples)-1].TS
	mid := time.Date(first.Year(), first.Month(), first.Day(), 12, 0, 0, 0, first.Location())
	return []string{first.Format("15:04"), mid.Format("15:04"), last.Format("15:04")}
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
