package ui

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
)

// Braille dot bits indexed by [row][column] within one character cell. The
// grid is two dots wide and four dots tall.
var brailleBits = [4][2]rune{
	{0x01, 0x08},
	{0x02, 0x10},
	{0x04, 0x20},
	{0x40, 0x80},
}

type chartCell struct {
	bits     rune
	color    color.Color
	priority int
}

// LineSeries is one plotted series. Values are indexed positions; nil values
// are gaps. The x axis is the index.
type LineSeries struct {
	Name   string
	Color  color.Color
	Values []*float64
}

// LineOptions configures a braille line chart.
type LineOptions struct {
	Width, Height int
	Series        []LineSeries
	Cursor        int // index into values, or -1
	CursorColor   color.Color
	Dim           color.Color
	ShowYAxis     bool
	ShowXAxis     bool
	XLabels       []string // three labels: start, middle, end
	ZeroLine      bool
	MinY, MaxY    *float64 // optional fixed range
	YLabel        func(float64) string
}

// RenderLine renders a braille line chart as exactly Height lines.
func RenderLine(o LineOptions) []string {
	if o.Width < 4 || o.Height < 1 {
		return nil
	}
	axisW := 0
	if o.ShowYAxis {
		axisW = 8
	}
	plotW := o.Width - axisW
	if plotW < 4 {
		plotW = 4
		axisW = o.Width - plotW
		if axisW < 0 {
			axisW = 0
		}
	}
	plotH := o.Height
	if o.ShowXAxis {
		plotH--
	}
	if plotH < 1 {
		plotH = 1
	}

	minY, maxY := chartRange(o.Series)
	if o.MinY != nil {
		minY = *o.MinY
	}
	if o.MaxY != nil {
		maxY = *o.MaxY
	}
	if o.ZeroLine {
		if minY > 0 {
			minY = 0
		}
		if maxY < 0 {
			maxY = 0
		}
	}
	if maxY <= minY {
		maxY = minY + 1
	}
	if o.MinY == nil && o.MaxY == nil {
		pad := (maxY - minY) * 0.05
		minY -= pad
		maxY += pad
	}
	if o.ZeroLine && minY > 0 {
		minY = 0
	}

	pw := plotW * 2
	ph := plotH * 4
	cells := make(map[[2]int]chartCell, plotW*plotH)

	plot := func(px, py int, c color.Color, priority int) {
		if px < 0 || py < 0 || px >= pw || py >= ph {
			return
		}
		cx, cy := px/2, py/4
		key := [2]int{cx, cy}
		bit := brailleBits[py%4][px%2]
		if existing, ok := cells[key]; ok {
			if priority < existing.priority {
				cells[key] = chartCell{bits: existing.bits | bit, color: c, priority: priority}
			} else {
				existing.bits |= bit
				cells[key] = existing
			}
			return
		}
		cells[key] = chartCell{bits: bit, color: c, priority: priority}
	}

	n := 0
	for _, s := range o.Series {
		if len(s.Values) > n {
			n = len(s.Values)
		}
	}
	if n == 0 {
		return emptyChart(o)
	}
	xFor := func(i int) int {
		if n == 1 {
			return pw / 2
		}
		return int(math.Round(float64(i) / float64(n-1) * float64(pw-1)))
	}
	yFor := func(v float64) int {
		return int(math.Round((maxY - v) / (maxY - minY) * float64(ph-1)))
	}

	if o.ZeroLine && minY < 0 && maxY > 0 {
		zy := yFor(0)
		for px := 0; px < pw; px++ {
			plot(px, zy, o.Dim, 100)
		}
	}

	for si, s := range o.Series {
		prevX, prevY := -1, -1
		for i, v := range s.Values {
			if v == nil {
				prevX, prevY = -1, -1
				continue
			}
			px, py := xFor(i), yFor(*v)
			if prevX >= 0 {
				drawLine(plot, prevX, prevY, px, py, s.Color, si)
			}
			plot(px, py, s.Color, si)
			prevX, prevY = px, py
		}
	}

	if o.Cursor >= 0 && o.Cursor < n {
		cx := xFor(o.Cursor) / 2
		for cy := 0; cy < plotH; cy++ {
			key := [2]int{cx, cy}
			existing := cells[key]
			cells[key] = chartCell{bits: existing.bits, color: o.CursorColor, priority: -10}
		}
	}

	lines := make([]string, 0, o.Height)
	yLabel := o.YLabel
	if yLabel == nil {
		yLabel = axisPower
	}
	for row := 0; row < plotH; row++ {
		var b strings.Builder
		if axisW > 0 {
			label := ""
			switch row {
			case 0:
				label = yLabel(maxY)
			case plotH / 2:
				if plotH > 4 {
					label = yLabel((maxY + minY) / 2)
				}
			case plotH - 1:
				label = yLabel(minY)
			}
			b.WriteString(padLeft(label, axisW-1))
			b.WriteString(" ")
		}
		for col := 0; col < plotW; col++ {
			cell, ok := cells[[2]int{col, row}]
			if !ok || cell.bits == 0 {
				b.WriteString(" ")
				continue
			}
			r := rune(0x2800) + cell.bits
			if cell.color != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(cell.color).Render(string(r)))
			} else {
				b.WriteRune(r)
			}
		}
		lines = append(lines, b.String())
	}

	if o.ShowXAxis {
		labels := o.XLabels
		if len(labels) < 3 {
			labels = []string{"", "", ""}
		}
		var b strings.Builder
		if axisW > 0 {
			b.WriteString(strings.Repeat(" ", axisW))
		}
		b.WriteString(fitXLabels(labels, plotW))
		lines = append(lines, b.String())
	}
	return lines
}

func drawLine(plot func(int, int, color.Color, int), x0, y0, x1, y1 int, c color.Color, prio int) {
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		plot(x0, y0, c, prio)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func chartRange(series []LineSeries) (minY, maxY float64) {
	first := true
	for _, s := range series {
		for _, v := range s.Values {
			if v == nil {
				continue
			}
			if first {
				minY, maxY = *v, *v
				first = false
				continue
			}
			minY = math.Min(minY, *v)
			maxY = math.Max(maxY, *v)
		}
	}
	if first {
		return 0, 1
	}
	if minY == maxY {
		if minY == 0 {
			maxY = 1
		} else {
			minY *= 0.9
			maxY *= 1.1
		}
	}
	return minY, maxY
}

func emptyChart(o LineOptions) []string {
	out := make([]string, 0, o.Height)
	for i := 0; i < o.Height; i++ {
		if i == o.Height/2 {
			out = append(out, centerText("no data yet", o.Width))
		} else {
			out = append(out, strings.Repeat(" ", o.Width))
		}
	}
	return out
}

func fitXLabels(labels []string, w int) string {
	if w <= 0 {
		return ""
	}
	start, mid, end := labels[0], labels[1], labels[2]
	midW := len([]rune(mid))
	line := make([]rune, w)
	for i := range line {
		line[i] = ' '
	}
	put := func(s string, at int) {
		for i, r := range []rune(s) {
			pos := at + i
			if pos >= 0 && pos < w {
				line[pos] = r
			}
		}
	}
	put(start, 0)
	put(mid, (w-midW)/2)
	put(end, w-len([]rune(end)))
	return string(line)
}

func axisPower(v float64) string {
	a := math.Abs(v)
	switch {
	case a >= 1_000_000:
		return trimNum(v/1_000_000) + "MW"
	case a >= 1000:
		return trimNum(v/1000) + "kW"
	default:
		return trimNum(v) + "W"
	}
}

func trimNum(v float64) string {
	if math.Abs(v) >= 10 {
		return strings.TrimSuffix(fmtFixed(v, 0), ".0")
	}
	return strings.TrimSuffix(fmtFixed(v, 1), ".0")
}

func fmtFixed(v float64, prec int) string {
	return strconv.FormatFloat(v, 'f', prec, 64)
}

// Sparkline renders values as block characters across width cells.
func Sparkline(values []*float64, width int, c color.Color) string {
	if width <= 0 {
		return ""
	}
	if len(values) == 0 {
		return strings.Repeat(" ", width)
	}
	if len(values) > width {
		values = values[len(values)-width:]
	}
	var minV, maxV float64
	first := true
	for _, v := range values {
		if v == nil {
			continue
		}
		if first {
			minV, maxV = *v, *v
			first = false
			continue
		}
		minV = math.Min(minV, *v)
		maxV = math.Max(maxV, *v)
	}
	if first {
		return strings.Repeat(" ", width)
	}
	if maxV == minV {
		maxV = minV + 1
	}
	blocks := []rune("▁▂▃▄▅▆▇█")
	var b strings.Builder
	for _, v := range values {
		if v == nil {
			b.WriteString(" ")
			continue
		}
		x := (*v - minV) / (maxV - minV)
		idx := int(math.Round(x * float64(len(blocks)-1)))
		idx = clamp(idx, 0, len(blocks)-1)
		r := string(blocks[idx])
		if c != nil {
			b.WriteString(lipgloss.NewStyle().Foreground(c).Render(r))
		} else {
			b.WriteString(r)
		}
	}
	if len(values) < width {
		b.WriteString(strings.Repeat(" ", width-len(values)))
	}
	return b.String()
}

// Bar is one column of a bar chart.
type Bar struct {
	Label     string
	Value     float64
	Color     color.Color
	Highlight bool
}

// BarOptions configures a vertical bar chart.
type BarOptions struct {
	Width, Height int
	Bars          []Bar
	Dim           color.Color
	Fg            color.Color
	ShowYAxis     bool
	LabelStep     int
}

// RenderBars renders a vertical bar chart as exactly Height lines.
func RenderBars(o BarOptions) []string {
	if o.Width < 4 || o.Height < 2 || len(o.Bars) == 0 {
		return emptyBars(o)
	}
	axisW := 0
	if o.ShowYAxis {
		axisW = 8
	}
	plotW := o.Width - axisW
	plotH := o.Height - 1
	if plotH < 1 {
		plotH = 1
	}
	if plotW < 1 {
		plotW = 1
	}

	maxV := 0.0
	for _, b := range o.Bars {
		maxV = math.Max(maxV, b.Value)
	}
	if maxV <= 0 {
		maxV = 1
	}

	n := len(o.Bars)
	barW := plotW / n
	gap := 0
	if barW >= 3 {
		barW--
		gap = 1
	}
	if barW < 1 {
		barW = 1
	}

	// grid[row][col] holds a coloured block, if any.
	type cell struct {
		r rune
		c color.Color
	}
	grid := make([][]cell, plotH)
	for i := range grid {
		grid[i] = make([]cell, plotW)
	}
	eighthBlocks := []rune("▁▂▃▄▅▆▇█")

	for bi, bar := range o.Bars {
		x0 := bi * (barW + gap)
		if x0 >= plotW {
			break
		}
		color := bar.Color
		if bar.Highlight && o.Fg != nil {
			color = o.Fg
		}
		totalEighths := int(math.Round(bar.Value / maxV * float64(plotH*8)))
		if totalEighths <= 0 && bar.Value > 0 {
			totalEighths = 1
		}
		full := totalEighths / 8
		rem := totalEighths % 8
		for x := x0; x < x0+barW && x < plotW; x++ {
			for row := 0; row < full && row < plotH; row++ {
				grid[plotH-1-row][x] = cell{r: '█', c: color}
			}
			if rem > 0 && full < plotH {
				grid[plotH-1-full][x] = cell{r: eighthBlocks[rem-1], c: color}
			}
		}
	}

	var lines []string
	for row := 0; row < plotH; row++ {
		var b strings.Builder
		if axisW > 0 {
			label := ""
			switch row {
			case 0:
				label = trimNum(maxV)
			case plotH / 2:
				if plotH > 4 {
					label = trimNum(maxV / 2)
				}
			case plotH - 1:
				label = "0"
			}
			b.WriteString(padLeft(label, axisW-1))
			b.WriteString(" ")
		}
		for col := 0; col < plotW; col++ {
			cl := grid[row][col]
			if cl.r == 0 {
				b.WriteString(" ")
				continue
			}
			if cl.c != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(cl.c).Render(string(cl.r)))
			} else {
				b.WriteRune(cl.r)
			}
		}
		lines = append(lines, b.String())
	}

	// x labels
	step := o.LabelStep
	if step <= 0 {
		step = max(1, n/8)
	}
	labelLine := make([]rune, o.Width)
	for i := range labelLine {
		labelLine[i] = ' '
	}
	offset := axisW
	for bi, bar := range o.Bars {
		if bi%step != 0 && bi != n-1 {
			continue
		}
		x0 := offset + bi*(barW+gap)
		text := []rune(bar.Label)
		start := x0 + (barW-len(text))/2
		if start < offset {
			start = offset
		}
		for i, r := range text {
			pos := start + i
			if pos >= 0 && pos < o.Width {
				labelLine[pos] = r
			}
		}
	}
	lines = append(lines, string(labelLine))
	return lines
}

func emptyBars(o BarOptions) []string {
	out := make([]string, 0, o.Height)
	for i := 0; i < o.Height; i++ {
		if i == o.Height/2 {
			out = append(out, centerText("no history yet", o.Width))
		} else {
			out = append(out, strings.Repeat(" ", o.Width))
		}
	}
	return out
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
