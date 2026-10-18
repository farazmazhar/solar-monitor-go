package ui

import (
	"image/color"
	"strings"
	"time"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

func (m *Model) renderLive(w, h, y int) string {
	flowH := 8
	switch {
	case w < 66:
		flowH = 3
	case h < 20:
		flowH = 6
	}
	var lines []string
	lines = append(lines, renderFlow(w, flowH, m.snap, m.th)...)
	lines = fitLines(lines, w, flowH)[:flowH]

	lines = append(lines, divider(m.th, w))
	tiles := m.renderTiles(w, y+len(lines))
	lines = append(lines, tiles...)
	lines = append(lines, divider(m.th, w))
	lines = append(lines, m.renderGauge(w)...)

	remaining := h - len(lines)
	if remaining >= 3 {
		lines = append(lines, divider(m.th, w))
		chartY := y + len(lines)
		chart := m.renderMiniChart(w, remaining-1, chartY)
		lines = append(lines, chart...)
	}
	return strings.Join(fitLines(lines, w, h), "\n")
}

func fitLines(lines []string, w, h int) []string {
	for i := range lines {
		lines[i] = padRight(lines[i], w)
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	return lines
}

func (m *Model) renderMiniChart(w, h, y int) []string {
	peak := 0.0
	peakAt := time.Time{}
	for _, s := range m.daySamples {
		if s.SolarW != nil && *s.SolarW > peak {
			peak = *s.SolarW
			peakAt = s.TS
		}
	}
	title := m.th.Label.Render("TODAY") + "  " +
		m.th.Style(m.th.Solar).Render(fmtEnergy(m.snap.TodayKWh)) + m.th.Muted.Render(" generated") +
		m.th.Muted.Render("   peak ") + m.th.Style(m.th.Solar).Render(fmtPowerAbs(peak))
	if !peakAt.IsZero() {
		title += m.th.Muted.Render(" at " + peakAt.Format("15:04"))
	}
	if n := m.chartCursor; n >= 0 && n < len(m.daySamples) {
		s := m.daySamples[n]
		title += m.th.Muted.Render("   cursor "+s.TS.Format("15:04")+"  ") +
			m.th.Style(m.th.Solar).Render(fmtPower(s.SolarW))
	}
	out := []string{title}
	chartH := h - 1
	series := m.miniSeries()
	chart := RenderLine(LineOptions{
		Width: w, Height: chartH, Series: series,
		Cursor: m.chartCursor, CursorColor: m.th.Fg, Dim: m.th.Faint,
		ZeroLine: false,
	})
	out = append(out, chart...)
	m.hits.Add(Rect{X: 0, Y: y + 1, W: w, H: max(1, chartH)}, Action{Kind: ActionChartCursor, Rect: Rect{X: 0, Y: y + 1, W: w, H: chartH}})
	return out
}

// miniSeries builds a lighter two-series chart for the Live screen.
func (m *Model) miniSeries() []LineSeries {
	n := len(m.daySamples)
	if n == 0 {
		return nil
	}
	solar := make([]*float64, n)
	load := make([]*float64, n)
	for i, s := range m.daySamples {
		solar[i] = s.SolarW
		load[i] = s.LoadW
	}
	return []LineSeries{
		{Name: "solar", Color: m.th.Solar, Values: solar},
		{Name: "load", Color: m.th.Fg, Values: load},
	}
}

// todaySeries builds the chart series from the local day samples, respecting
// the per-series toggles.
func (m *Model) todaySeries() []LineSeries {
	n := len(m.daySamples)
	if n == 0 {
		return nil
	}
	mk := func(name string, c color.Color, pick func(provider.Sample) *float64) LineSeries {
		vals := make([]*float64, n)
		for i, s := range m.daySamples {
			vals[i] = pick(s)
		}
		return LineSeries{Name: name, Color: c, Values: vals}
	}
	var out []LineSeries
	if m.series[0] {
		out = append(out, mk("solar", m.th.Solar, func(s provider.Sample) *float64 { return s.SolarW }))
	}
	if m.series[1] {
		out = append(out, mk("load", m.th.Fg, func(s provider.Sample) *float64 { return s.LoadW }))
	}
	if m.series[2] {
		out = append(out, mk("grid", m.th.Grid, func(s provider.Sample) *float64 { return s.GridW }))
	}
	if m.series[3] {
		out = append(out, mk("battery", m.th.Battery, func(s provider.Sample) *float64 { return s.BatteryW }))
	}
	return out
}
