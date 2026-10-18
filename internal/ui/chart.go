package ui

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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
	bits     rune  // braille dots, when the style is braille
	r        rune  // explicit glyph, e.g. the cursor
	halves   uint8 // solid style: bit 1 = top half, bit 2 = bottom half
	color    color.Color
	priority int
}

// LineSeries is one plotted series. Values are indexed positions; nil values
// are gaps. The x axis is the index unless the caller fills a fixed day window.
type LineSeries struct {
	Name   string
	Color  color.Color
	Values []*float64
}

// ChartStyle selects how line series are drawn.
type ChartStyle int

const (
	// ChartSolid connects points with box-drawing lines, which read as solid.
	ChartSolid ChartStyle = iota
	// ChartBraille draws the denser braille pixel graph.
	ChartBraille
)

// LineOptions configures a line chart.
type LineOptions struct {
	Width, Height int

	// Title and Border draw a rounded panel around the chart.
	Title       string
	Border      bool
	BorderColor color.Color
	TitleColor  color.Color

	// Style is ChartSolid by default.
	Style ChartStyle

	Series      []LineSeries
	Cursor      int // index into values, or -1
	CursorColor color.Color
	Dim         color.Color
	Fg          color.Color
	ShowYAxis   bool
	ShowXAxis   bool
	XLabels     []string // distributed evenly across the plot width
	ZeroLine    bool
	MinY, MaxY  *float64 // optional fixed range
	YLabel      func(float64) string
}

// RenderLine renders a line chart as exactly Height lines. The second return
// value is the plot rectangle in local coordinates, for mouse hits.
func RenderLine(o LineOptions) ([]string, Rect) {
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
	if contentW < 4 || contentH < 1 {
		return nil, Rect{}
	}

	axisW := 0
	if o.ShowYAxis {
		axisW = 8
	}
	plotW := contentW - axisW
	if plotW < 4 {
		plotW = 4
		axisW = contentW - plotW
		if axisW < 0 {
			axisW = 0
		}
	}
	xRows := 0
	if o.ShowXAxis {
		xRows = 1
	}
	plotH := contentH - xRows
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
		pad := (maxY - minY) * 0.06
		minY -= pad
		maxY += pad
	}
	if o.ZeroLine && minY > 0 {
		minY = 0
	}

	n := 0
	for _, s := range o.Series {
		if len(s.Values) > n {
			n = len(s.Values)
		}
	}
	if n == 0 {
		return emptyPanel(o, contentW, contentH, axisW, plotH, xRows), Rect{}
	}

	cells := make(map[[2]int]chartCell, plotW*plotH)
	if o.Style == ChartBraille {
		plotBraille(cells, o, minY, maxY, plotW, plotH, n)
	} else {
		plotSolid(cells, o, minY, maxY, plotW, plotH, n)
	}

	zeroRow := -1
	if o.ZeroLine && minY < 0 && maxY > 0 {
		zeroRow = yHalfFor(0, minY, maxY, plotH) / 2
	}

	if o.Cursor >= 0 && o.Cursor < n {
		if o.Style == ChartBraille {
			pw := plotW * 2
			cx := brailleXFor(o.Cursor, n, pw) / 2
			for cy := 0; cy < plotH; cy++ {
				key := [2]int{cx, cy}
				existing := cells[key]
				cells[key] = chartCell{bits: existing.bits, color: o.CursorColor, priority: -10}
			}
		} else {
			cx := solidXFor(o.Cursor, n, plotW)
			for cy := 0; cy < plotH; cy++ {
				cells[[2]int{cx, cy}] = chartCell{r: '┃', color: o.CursorColor, priority: -10}
			}
		}
	}

	yLabel := o.YLabel
	if yLabel == nil {
		yLabel = axisPower
	}

	var content []string
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
			if !ok || (cell.bits == 0 && cell.r == 0 && cell.halves == 0) {
				if row == zeroRow {
					if o.Dim != nil {
						b.WriteString(lipgloss.NewStyle().Foreground(o.Dim).Render("╌"))
					} else {
						b.WriteString("╌")
					}
				} else {
					b.WriteString(" ")
				}
				continue
			}
			r := cell.r
			switch {
			case o.Style == ChartBraille:
				r = rune(0x2800) + cell.bits
			case r == 0:
				r = halfBlock(cell.halves)
			}
			if cell.color != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(cell.color).Render(string(r)))
			} else {
				b.WriteRune(r)
			}
		}
		content = append(content, b.String())
	}

	if xRows == 1 {
		var b strings.Builder
		if axisW > 0 {
			b.WriteString(strings.Repeat(" ", axisW))
		}
		b.WriteString(distributeLabels(o.XLabels, plotW))
		content = append(content, b.String())
	}

	hit := Rect{X: offset + axisW, Y: offset, W: plotW, H: plotH}
	if !border {
		return padLines(content, o.Width), hit
	}
	return wrapPanel(o.Title, content, o.Width, o.BorderColor, o.TitleColor, o.Dim), hit
}

// solidXFor maps a value index to a cell column.
func solidXFor(i, n, plotW int) int {
	if n <= 1 {
		return plotW / 2
	}
	return int(math.Round(float64(i) / float64(n-1) * float64(plotW-1)))
}

// yHalfFor maps a value to a half-cell row (two sub-rows per cell).
func yHalfFor(v, minY, maxY float64, plotH int) int {
	if maxY <= minY {
		return 0
	}
	rows := plotH * 2
	h := int(math.Round((maxY - v) / (maxY - minY) * float64(rows-1)))
	if h < 0 {
		return 0
	}
	if h > rows-1 {
		return rows - 1
	}
	return h
}

// halfBlock turns a pair of half-cell bits into a block glyph.
func halfBlock(halves uint8) rune {
	switch halves {
	case 1:
		return '▀'
	case 2:
		return '▄'
	case 3:
		return '█'
	default:
		return ' '
	}
}

// brailleXFor maps a value index to a braille pixel column.
func brailleXFor(i, n, pw int) int {
	if n <= 1 {
		return pw / 2
	}
	return int(math.Round(float64(i) / float64(n-1) * float64(pw-1)))
}

// brailleYFor maps a value to a braille pixel row.
func brailleYFor(v, minY, maxY float64, ph int) int {
	if maxY <= minY {
		return 0
	}
	return int(math.Round((maxY - v) / (maxY - minY) * float64(ph-1)))
}

// plotSolid draws solid half-block lines. Each column collects the vertical
// range its samples cover, which keeps steep changes connected.
func plotSolid(cells map[[2]int]chartCell, o LineOptions, minY, maxY float64, plotW, plotH, n int) {
	mark := func(col, half int, c color.Color, priority int) {
		if col < 0 || half < 0 || col >= plotW || half >= plotH*2 {
			return
		}
		bit := uint8(1) << uint(half%2)
		key := [2]int{col, half / 2}
		existing, ok := cells[key]
		if !ok {
			cells[key] = chartCell{halves: bit, color: c, priority: priority}
			return
		}
		if priority < existing.priority {
			cells[key] = chartCell{halves: existing.halves | bit, color: c, priority: priority}
			return
		}
		existing.halves |= bit
		cells[key] = existing
	}

	for si, s := range o.Series {
		type span struct {
			min, max int
			set      bool
		}
		spans := make([]span, plotW)
		for i, v := range s.Values {
			if v == nil {
				continue
			}
			col := solidXFor(i, n, plotW)
			half := yHalfFor(*v, minY, maxY, plotH)
			sp := &spans[col]
			if !sp.set {
				sp.min, sp.max, sp.set = half, half, true
				continue
			}
			if half < sp.min {
				sp.min = half
			}
			if half > sp.max {
				sp.max = half
			}
		}
		for col, sp := range spans {
			if !sp.set {
				continue
			}
			for half := sp.min; half <= sp.max; half++ {
				mark(col, half, s.Color, si)
			}
		}
	}
}

// plotBraille fills the cell map with braille dots.
func plotBraille(cells map[[2]int]chartCell, o LineOptions, minY, maxY float64, plotW, plotH, n int) {
	pw := plotW * 2
	ph := plotH * 4
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
	for si, s := range o.Series {
		prevX, prevY := -1, -1
		for i, v := range s.Values {
			if v == nil {
				prevX, prevY = -1, -1
				continue
			}
			px := brailleXFor(i, n, pw)
			py := brailleYFor(*v, minY, maxY, ph)
			if prevX >= 0 {
				drawLine(plot, prevX, prevY, px, py, s.Color, si)
			}
			plot(px, py, s.Color, si)
			prevX, prevY = px, py
		}
	}
}

func emptyPanel(o LineOptions, contentW, contentH, axisW, plotH, xRows int) []string {
	lines := make([]string, 0, contentH)
	for i := 0; i < plotH; i++ {
		if i == plotH/2 {
			var b strings.Builder
			if axisW > 0 {
				b.WriteString(strings.Repeat(" ", axisW))
			}
			b.WriteString(centerText("no data yet", contentW-axisW))
			lines = append(lines, b.String())
		} else {
			lines = append(lines, strings.Repeat(" ", contentW))
		}
	}
	if xRows == 1 {
		lines = append(lines, strings.Repeat(" ", contentW))
	}
	if !o.Border {
		return padLines(lines, o.Width)
	}
	return wrapPanel(o.Title, lines, o.Width, o.BorderColor, o.TitleColor, o.Dim)
}

// wrapPanel draws a rounded border with an optional title in the top edge.
func wrapPanel(title string, content []string, width int, borderColor, titleColor, dim color.Color) []string {
	if width < 4 {
		width = 4
	}
	bc := borderColor
	if bc == nil {
		bc = dim
	}
	rule := lipgloss.NewStyle().Foreground(bc)
	bold := lipgloss.NewStyle().Bold(true).Foreground(titleColor)
	inner := width - 2

	out := make([]string, 0, len(content)+2)
	if title == "" {
		out = append(out, rule.Render("╭"+strings.Repeat("─", inner)+"╮"))
	} else {
		t := clampText(title, inner-3)
		fill := inner - ansi.StringWidth(t) - 3
		if fill < 0 {
			fill = 0
		}
		out = append(out, rule.Render("╭─ ")+bold.Render(t)+rule.Render(" "+strings.Repeat("─", fill)+"╮"))
	}
	for _, line := range content {
		out = append(out, rule.Render("│")+padRight(line, inner)+rule.Render("│"))
	}
	out = append(out, rule.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return out
}

func padLines(lines []string, width int) []string {
	for i := range lines {
		lines[i] = padRight(lines[i], width)
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

// distributeLabels places labels evenly across w cells, centred on their slot.
func distributeLabels(labels []string, w int) string {
	if w <= 0 || len(labels) == 0 {
		return strings.Repeat(" ", max(0, w))
	}
	line := make([]rune, w)
	for i := range line {
		line[i] = ' '
	}
	n := len(labels)
	for i, label := range labels {
		x := 0
		if n > 1 {
			x = int(math.Round(float64(i) * float64(w-1) / float64(n-1)))
		}
		lw := ansi.StringWidth(label)
		start := x - lw/2
		if i == 0 {
			start = x
		}
		if i == n-1 {
			start = x - lw + 1
		}
		if start < 0 {
			start = 0
		}
		if start+lw > w {
			start = w - lw
		}
		for j, r := range label {
			if start+j >= 0 && start+j < w {
				line[start+j] = r
			}
		}
	}
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
		return strconv.FormatFloat(v, 'f', 0, 64)
	}
	return strconv.FormatFloat(v, 'f', 1, 64)
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
	Title         string
	Border        bool
	BorderColor   color.Color
	TitleColor    color.Color
	Bars          []Bar
	Dim           color.Color
	Fg            color.Color
	ShowYAxis     bool
	LabelStep     int
	YLabel        func(float64) string
}

// RenderBars renders a vertical bar chart as exactly Height lines.
func RenderBars(o BarOptions) []string {
	if o.Width < 6 || o.Height < 3 || len(o.Bars) == 0 {
		return emptyBars(o)
	}
	border := o.Border
	contentW, contentH := o.Width, o.Height
	if border {
		contentW -= 2
		contentH -= 2
	}
	axisW := 0
	if o.ShowYAxis {
		axisW = 8
	}
	plotW := contentW - axisW
	xRows := 1
	plotH := contentH - xRows
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
		barColor := bar.Color
		if bar.Highlight && o.Fg != nil {
			barColor = o.Fg
		}
		totalEighths := int(math.Round(bar.Value / maxV * float64(plotH*8)))
		if totalEighths <= 0 && bar.Value > 0 {
			totalEighths = 1
		}
		full := totalEighths / 8
		rem := totalEighths % 8
		for x := x0; x < x0+barW && x < plotW; x++ {
			for row := 0; row < full && row < plotH; row++ {
				grid[plotH-1-row][x] = cell{r: '█', c: barColor}
			}
			if rem > 0 && full < plotH {
				grid[plotH-1-full][x] = cell{r: eighthBlocks[rem-1], c: barColor}
			}
		}
	}

	yLabel := o.YLabel
	if yLabel == nil {
		yLabel = trimNum
	}

	var content []string
	for row := 0; row < plotH; row++ {
		var b strings.Builder
		if axisW > 0 {
			label := ""
			switch row {
			case 0:
				label = yLabel(maxV)
			case plotH / 2:
				if plotH > 4 {
					label = yLabel(maxV / 2)
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
		content = append(content, b.String())
	}

	// x labels
	step := o.LabelStep
	if step <= 0 {
		step = max(1, n/8)
	}
	labelLine := make([]rune, contentW)
	for i := range labelLine {
		labelLine[i] = ' '
	}
	for bi, bar := range o.Bars {
		if bi%step != 0 && bi != n-1 {
			continue
		}
		x0 := axisW + bi*(barW+gap)
		text := []rune(bar.Label)
		start := x0 + (barW-len(text))/2
		if start < axisW {
			start = axisW
		}
		for i, r := range text {
			pos := start + i
			if pos >= 0 && pos < contentW {
				labelLine[pos] = r
			}
		}
	}
	content = append(content, string(labelLine))

	if !border {
		return padLines(content, o.Width)
	}
	return wrapPanel(o.Title, content, o.Width, o.BorderColor, o.TitleColor, o.Dim)
}

func emptyBars(o BarOptions) []string {
	contentH := o.Height
	contentW := o.Width
	if o.Border {
		contentH -= 2
		contentW -= 2
	}
	if contentH < 1 {
		contentH = 1
	}
	lines := make([]string, 0, contentH)
	for i := 0; i < contentH; i++ {
		if i == contentH/2 {
			lines = append(lines, centerText("no history yet", contentW))
		} else {
			lines = append(lines, strings.Repeat(" ", contentW))
		}
	}
	if !o.Border {
		return padLines(lines, o.Width)
	}
	return wrapPanel(o.Title, lines, o.Width, o.BorderColor, o.TitleColor, o.Dim)
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
