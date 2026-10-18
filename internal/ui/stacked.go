package ui

import (
	"image/color"
	"math"
	"strings"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

// StackSeries is one layer of a stacked-area chart, in bottom-to-top order.
type StackSeries struct {
	Name   string
	Color  color.Color
	Glyph  rune
	Values []*float64
}

// StackOptions configures a stacked-area chart.
type StackOptions struct {
	Width, Height int
	Title         string
	Border        bool
	BorderColor   color.Color
	TitleColor    color.Color
	Series        []StackSeries
	Cursor        int
	CursorColor   color.Color
	Dim           color.Color
	Fg            color.Color
	XLabels       []string
	YLabel        func(float64) string
}

// RenderStack draws a stacked source-mix chart with axes and a border.
func RenderStack(o StackOptions) ([]string, Rect) {
	if o.Width < 6 || o.Height < 3 {
		return nil, Rect{}
	}
	border := o.Border
	contentW, contentH := o.Width, o.Height
	offset := 0
	if border {
		contentW -= 2
		contentH -= 2
		offset = 1
	}
	axisW := 8
	xRows := 1
	plotW := contentW - axisW
	plotH := contentH - xRows
	if plotW < 4 {
		plotW = 4
	}
	if plotH < 1 {
		plotH = 1
	}

	n := len(o.Series[0].Values)
	if n == 0 {
		return emptyStack(o, contentW, contentH, axisW, plotH), Rect{}
	}

	// Total stack per column and the global maximum.
	totals := make([]float64, n)
	maxTotal := 0.0
	for col := 0; col < n; col++ {
		t := 0.0
		for _, s := range o.Series {
			if col < len(s.Values) && s.Values[col] != nil {
				t += math.Max(0, *s.Values[col])
			}
		}
		totals[col] = t
		maxTotal = math.Max(maxTotal, t)
	}
	if maxTotal <= 0 {
		maxTotal = 1
	}
	maxTotal *= 1.04

	type cell struct {
		r     rune
		color color.Color
	}
	grid := make([][]cell, plotH)
	for i := range grid {
		grid[i] = make([]cell, plotW)
	}

	for col := 0; col < plotW; col++ {
		// Map column to sample index.
		idx := col
		if n != plotW {
			idx = int(math.Round(float64(col) / float64(plotW-1) * float64(n-1)))
		}
		if idx < 0 || idx >= n {
			continue
		}
		total := totals[idx]
		if total <= 0 {
			continue
		}
		// cumulative values per series
		acc := 0.0
		for _, s := range o.Series {
			if idx >= len(s.Values) || s.Values[idx] == nil {
				continue
			}
			v := math.Max(0, *s.Values[idx])
			bottom := acc / maxTotal * float64(plotH)
			top := (acc + v) / maxTotal * float64(plotH)
			acc += v
			// Fill rows from the bottom of the stack up to the top.
			for r := 0; r < plotH; r++ {
				// row r spans [plotH-1-r, plotH-r) cells from the bottom
				rBottom := float64(plotH-1-r) + 0.5
				if rBottom >= bottom && rBottom < top {
					glyph := s.Glyph
					if glyph == 0 {
						glyph = '█'
					}
					grid[r][col] = cell{r: glyph, color: s.Color}
				}
			}
		}
	}

	if o.Cursor >= 0 && o.Cursor < n {
		cx := int(math.Round(float64(o.Cursor) / float64(n-1) * float64(plotW-1)))
		for r := 0; r < plotH; r++ {
			grid[r][cx] = cell{r: '┃', color: o.CursorColor}
		}
	}

	yLabel := o.YLabel
	if yLabel == nil {
		yLabel = axisPower
	}
	var content []string
	for r := 0; r < plotH; r++ {
		var b strings.Builder
		label := ""
		switch r {
		case 0:
			label = yLabel(maxTotal)
		case plotH / 2:
			if plotH > 4 {
				label = yLabel(maxTotal / 2)
			}
		case plotH - 1:
			label = "0"
		}
		b.WriteString(padLeft(label, axisW-1))
		b.WriteString(" ")
		for col := 0; col < plotW; col++ {
			cl := grid[r][col]
			if cl.r == 0 {
				b.WriteString(" ")
				continue
			}
			if cl.color != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(cl.color).Render(string(cl.r)))
			} else {
				b.WriteRune(cl.r)
			}
		}
		content = append(content, b.String())
	}

	var b strings.Builder
	b.WriteString(strings.Repeat(" ", axisW))
	b.WriteString(distributeLabels(o.XLabels, plotW))
	content = append(content, b.String())

	hit := Rect{X: offset + axisW, Y: offset, W: plotW, H: plotH}
	if !border {
		return padLines(content, o.Width), hit
	}
	return wrapPanel(o.Title, content, o.Width, o.BorderColor, o.TitleColor, o.Dim), hit
}

func emptyStack(o StackOptions, contentW, contentH, axisW, plotH int) []string {
	lines := make([]string, 0, contentH)
	for i := 0; i < plotH; i++ {
		if i == plotH/2 {
			lines = append(lines, strings.Repeat(" ", axisW)+centerText("no source data", contentW-axisW))
		} else {
			lines = append(lines, strings.Repeat(" ", contentW))
		}
	}
	lines = append(lines, strings.Repeat(" ", contentW))
	if !o.Border {
		return padLines(lines, o.Width)
	}
	return wrapPanel(o.Title, lines, o.Width, o.BorderColor, o.TitleColor, o.Dim)
}

// mixSeries turns samples into the three load-supply layers: grid at the
// bottom, battery in the middle, solar on top.
func (m *Model) mixSeries(samples []provider.Sample, fixed bool, day time.Time) []StackSeries {
	n := len(samples)
	if fixed {
		n = daySlotCount
	}
	solar := make([]*float64, n)
	battery := make([]*float64, n)
	grid := make([]*float64, n)

	for i, s := range samples {
		idx := i
		if fixed {
			slot := s.TS.In(day.Location()).Hour()*12 + s.TS.In(day.Location()).Minute()/5
			if slot < 0 || slot >= daySlotCount {
				continue
			}
			idx = slot
		}
		load := math.Max(0, provider.Conv(s.LoadW))
		solarW := math.Max(0, provider.Conv(s.SolarW))
		batW := math.Max(0, -provider.Conv(s.BatteryW))
		solarToLoad := math.Min(solarW, load)
		remaining := load - solarToLoad
		batToLoad := math.Min(batW, remaining)
		gridToLoad := math.Max(0, remaining-batToLoad)

		solar[idx] = provider.Float(solarToLoad)
		battery[idx] = provider.Float(batToLoad)
		grid[idx] = provider.Float(gridToLoad)
	}

	return []StackSeries{
		{Name: "grid", Color: m.th.Import, Glyph: '▒', Values: grid},
		{Name: "battery", Color: m.th.Discharge, Glyph: '▓', Values: battery},
		{Name: "solar", Color: m.th.Solar, Glyph: '█', Values: solar},
	}
}

// mixLegend shows the source contributions at the cursor.
func (m *Model) mixLegend(w int) string {
	s := m.selectedSample()
	if s == nil {
		return m.th.Muted.Render("waiting for data")
	}
	load := math.Max(0, provider.Conv(s.LoadW))
	solarW := math.Max(0, provider.Conv(s.SolarW))
	batW := math.Max(0, -provider.Conv(s.BatteryW))
	solarToLoad := math.Min(solarW, load)
	remaining := load - solarToLoad
	batToLoad := math.Min(batW, remaining)
	gridToLoad := math.Max(0, remaining-batToLoad)

	entries := []struct {
		name  string
		value float64
		c     color.Color
	}{
		{"solar", solarToLoad, m.th.Solar},
		{"battery", batToLoad, m.th.Discharge},
		{"grid", gridToLoad, m.th.Import},
	}
	var parts []string
	for _, e := range entries {
		parts = append(parts, styleFg(m.th, e.c, "● "+e.name+" ")+m.th.Muted.Render(fmtPowerAbs(e.value)))
	}
	parts = append(parts, m.th.Muted.Render("→ consumption "+fmtPowerAbs(load)))
	return clampText(strings.Join(parts, "   "), w)
}

// renderMixChart is the stacked-area variant of the day chart.
func (m *Model) renderMixChart(w, h int, samples []provider.Sample, day time.Time, fixed bool) ([]string, Rect) {
	cursor := -1
	xLabels := dayTimeLabels(day)
	if fixed {
		cursor = m.cursorSlot(day)
	} else {
		cursor = m.cursorSampleIndex(samples)
		xLabels = rollingTimeLabels(samples)
	}
	title := "SOURCE MIX"
	if fixed {
		title += " · FIXED DAY 00:00-23:59"
	} else {
		title += " · FIT SAMPLES"
	}
	return RenderStack(StackOptions{
		Width: w, Height: h,
		Title: title, Border: true,
		BorderColor: m.th.Faint, TitleColor: m.th.Accent,
		Series: m.mixSeries(samples, fixed, day),
		Cursor: cursor, CursorColor: m.th.Fg, Dim: m.th.Faint, Fg: m.th.Fg,
		XLabels: xLabels, YLabel: axisPower,
	})
}
