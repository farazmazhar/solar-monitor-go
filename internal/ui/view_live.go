package ui

import (
	"image/color"
	"os"
	"strings"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func (m *Model) renderLive(w, h, y int) string {
	flowH := 5
	if w >= 66 && h >= 20 {
		flowH = 8
	}
	if w < 66 {
		flowH = 3
	}
	var lines []string
	lines = append(lines, renderFlow(w, flowH, m.snap, m.th)...)
	lines = fitLines(lines, w, flowH)[:flowH]
	if h >= 24 {
		lines = append(lines, "")
	}

	// At 24 rows the chart needs the space more than the tile sparklines do.
	tileRows := 4
	switch {
	case h < 24:
		tileRows = 2
	case h < 26:
		tileRows = 3
	}
	tiles := m.renderTiles(w, y+len(lines), tileRows)
	lines = append(lines, tiles...)
	lines = append(lines, "")
	lines = append(lines, m.renderGaugeCompact(w))
	lines = append(lines, m.chartLegend(w))

	chartH := h - len(lines)
	if chartH < 5 {
		chartH = 5
	}
	chartY := y + len(lines)
	chart, hit := m.renderDayChart(w, chartH, "POWER · "+m.windowTitle(), m.activeSamples(), m.activeDay(), [4]bool{true, true, true, true}, m.staticDay, false)
	lines = append(lines, chart...)
	m.hits.Add(
		Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H},
		Action{Kind: ActionChartCursor, Rect: Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H}},
	)
	return strings.Join(fitLines(lines, w, h), "\n")
}

func (m *Model) windowTitle() string {
	if m.staticDay {
		return "FIXED DAY 00:00-23:59"
	}
	return "FIT SAMPLES"
}

// chartStyle resolves the configured line style.
func (m *Model) chartStyle() ChartStyle {
	style := strings.ToLower(strings.TrimSpace(m.cfg.Display.ChartStyle))
	if os.Getenv("NO_COLOR") != "" && (style == "" || style == "solid" || style == "area" || style == "graph" || style == "braille") {
		return ChartASCII
	}
	switch style {
	case "area":
		return ChartSolid
	case "box":
		return ChartBox
	case "braille":
		return ChartBraille
	case "quadrant":
		return ChartQuadrant
	case "ascii":
		return ChartASCII
	case "graph":
		return ChartGraph
	default:
		return ChartBraille
	}
}

// chartFill reports whether the generation area should be shaded, and which
// glyph to use for the fill.
func (m *Model) chartFill() (bool, rune) {
	if os.Getenv("NO_COLOR") != "" {
		return false, ' '
	}
	style := strings.ToLower(strings.TrimSpace(m.cfg.Display.ChartStyle))
	switch style {
	case "area":
		return true, '▒'
	case "", "solid", "graph", "braille":
		return true, '░'
	default:
		return false, ' '
	}
}

// renderDayChart draws the shared day chart: four power series, axes, border
// and a cursor. It returns the rendered lines and the plot hit rectangle
// relative to the block origin.
func (m *Model) renderDayChart(w, h int, title string, samples []provider.Sample, day time.Time, enabled [4]bool, fixed, zero bool) ([]string, Rect) {
	if m.chartView == ViewMix {
		return m.renderMixChart(w, h, samples, day, fixed)
	}
	series := m.seriesFor(samples, enabled, fixed)
	fill, fillGlyph := m.chartFill()
	cursor := -1
	xLabels := dayTimeLabels(day)
	if fixed {
		cursor = m.cursorSlot(day)
	} else {
		cursor = m.cursorSampleIndex(samples)
		xLabels = rollingTimeLabels(samples)
	}
	return RenderLine(LineOptions{
		Width: w, Height: h,
		Title:       title,
		Border:      true,
		BorderColor: m.th.Faint,
		TitleColor:  m.th.Accent,
		Style:       m.chartStyle(),
		Series:      series,
		Cursor:      cursor,
		CursorColor: m.th.Fg,
		Dim:         m.th.Faint,
		Fg:          m.th.Fg,
		ShowYAxis:   true,
		ShowXAxis:   true,
		XLabels:     xLabels,
		ZeroLine:    zero,
		Fill:        fill,
		FillSeries:  0,
		FillGlyph:   fillGlyph,
	})
}

func (m *Model) seriesFor(samples []provider.Sample, enabled [4]bool, fixed bool) []LineSeries {
	values := func(pick func(provider.Sample) *float64) []*float64 {
		if fixed {
			return daySlotValues(samples, pick)
		}
		return rollingValues(samples, pick)
	}
	var out []LineSeries
	if enabled[0] {
		out = append(out, LineSeries{Name: "solar", Color: m.th.Solar, Values: values(func(s provider.Sample) *float64 { return s.SolarW })})
	}
	if enabled[1] {
		out = append(out, LineSeries{Name: "load", Color: m.th.Fg, Values: values(func(s provider.Sample) *float64 { return s.LoadW })})
	}
	if enabled[2] {
		out = append(out, LineSeries{Name: "grid", Color: m.th.Grid, Values: values(func(s provider.Sample) *float64 { return s.GridW })})
	}
	if enabled[3] {
		out = append(out, LineSeries{Name: "battery", Color: m.th.Battery, Values: values(func(s provider.Sample) *float64 { return s.BatteryW })})
	}
	return out
}

func (m *Model) renderGaugeCompact(w int) string {
	line := m.renderGauge(w)
	if len(line) == 0 {
		return ""
	}
	return line[0]
}

func (m *Model) renderTiles(w, y, rows int) []string {
	tiles := m.tileData()
	if w < 80 {
		return m.renderTilesNarrow(tiles, w, y)
	}
	n := len(tiles)
	if n == 0 {
		return nil
	}
	gap := 2
	tileW := (w - gap*(n-1)) / n
	if tileW < 8 {
		tileW = 8
	}
	lines := make([]string, rows)
	x := 0
	for i, t := range tiles {
		switch rows {
		case 2:
			lines[0] += styleFg(m.th, t.color, padRight(clampText(t.label+"  "+t.value, tileW), tileW))
			lines[1] += m.th.Muted.Render(padRight(clampText(t.sub, tileW), tileW))
		case 3:
			lines[0] += styleFg(m.th, t.color, padRight(clampText(t.label+"  "+t.value, tileW), tileW))
			lines[1] += m.th.Muted.Render(padRight(clampText(t.sub, tileW), tileW))
			lines[2] += padRight(Sparkline(t.spark, tileW, t.color), tileW)
		default:
			lines[0] += styleFg(m.th, t.color, padRight(clampText(t.label, tileW), tileW))
			lines[1] += styleFg(m.th, t.color, padRight(clampText(t.value, tileW), tileW))
			lines[2] += m.th.Muted.Render(padRight(clampText(t.sub, tileW), tileW))
			lines[3] += padRight(Sparkline(t.spark, tileW, t.color), tileW)
		}
		m.hits.Add(Rect{X: x, Y: y, W: tileW, H: rows}, Action{Kind: ActionTile, Tile: t.kind})
		if i < n-1 {
			for j := range lines {
				lines[j] += strings.Repeat(" ", gap)
			}
			x += tileW + gap
		}
	}
	for i := range lines {
		lines[i] = padRight(lines[i], w)
	}
	return lines
}

func (m *Model) renderTilesNarrow(tiles []tile, w, y int) []string {
	if len(tiles) > 4 {
		tiles = tiles[:4]
	}
	gap := 2
	tileW := (w - gap) / 2
	var out []string
	for row := 0; row < 2; row++ {
		var l1, l2 strings.Builder
		for col := 0; col < 2; col++ {
			idx := row*2 + col
			if idx >= len(tiles) {
				l1.WriteString(strings.Repeat(" ", tileW))
				l2.WriteString(strings.Repeat(" ", tileW))
				continue
			}
			t := tiles[idx]
			l1.WriteString(styleFg(m.th, t.color, padRight(clampText(t.label+" "+t.value, tileW), tileW)))
			l2.WriteString(m.th.Muted.Render(padRight(clampText(t.sub, tileW), tileW)))
			m.hits.Add(Rect{X: col * (tileW + gap), Y: y + row*2, W: tileW, H: 2}, Action{Kind: ActionTile, Tile: t.kind})
			if col == 0 {
				l1.WriteString(strings.Repeat(" ", gap))
				l2.WriteString(strings.Repeat(" ", gap))
			}
		}
		out = append(out, padRight(l1.String(), w), padRight(l2.String(), w))
	}
	return out
}

// chartLegend dispatches to the power or source-mix key.
func (m *Model) chartLegend(w int) string {
	if m.chartView == ViewMix {
		return m.mixLegend(w)
	}
	return m.liveLegend(w)
}

// liveLegend is the colour key above the Live chart.
func (m *Model) liveLegend(w int) string {
	s := m.selectedSample()
	if s == nil {
		return m.th.Muted.Render("waiting for data")
	}
	entries := []struct {
		name  string
		value string
		c     color.Color
	}{
		{"solar", fmtPower(s.SolarW), m.th.Solar},
		{"consumption", fmtPower(s.LoadW), m.th.Fg},
		{"grid", fmtPower(s.GridW), m.th.Grid},
		{"battery", fmtPower(s.BatteryW), m.th.Battery},
	}
	var parts []string
	for _, e := range entries {
		parts = append(parts, styleFg(m.th, e.c, "● "+e.name+" ")+m.th.Muted.Render(e.value))
	}
	return clampText(strings.Join(parts, "   "), w)
}
