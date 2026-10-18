package ui

import (
	"image/color"
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	ncanvas "github.com/NimbleMarkets/ntcharts/v2/canvas"
	ngraph "github.com/NimbleMarkets/ntcharts/v2/canvas/graph"
	nrunes "github.com/NimbleMarkets/ntcharts/v2/canvas/runes"
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
	bits      uint16 // braille dots, quadrant bits or box-drawing directions
	r         rune   // explicit glyph, e.g. the cursor
	halves    uint8  // half-block style: bit 1 = top half, bit 2 = bottom half
	fill      bool   // area fill behind the line
	fillColor color.Color
	color     color.Color
	priority  int
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
	// ChartSolid connects points with half-block lines, which read as solid.
	ChartSolid ChartStyle = iota
	// ChartBox connects points with box-drawing lines and junctions.
	ChartBox
	// ChartBraille draws the dense braille pixel graph.
	ChartBraille
	// ChartQuadrant draws at 2x2 sub-cell resolution with quadrant blocks.
	ChartQuadrant
	// ChartASCII uses only ASCII glyphs for minimal fonts.
	ChartASCII
	// ChartGraph uses the ntcharts canvas graph line renderer.
	ChartGraph
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

	// Fill draws a shaded area below one series (0 is the first series).
	Fill       bool
	FillSeries int
	FillGlyph  rune
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
	if o.Fill {
		plotAreaFill(cells, o, minY, maxY, plotW, plotH, n)
	}
	switch o.Style {
	case ChartBraille:
		plotBraille(cells, o, minY, maxY, plotW, plotH, n)
	case ChartBox, ChartASCII:
		plotBox(cells, o, minY, maxY, plotW, plotH, n)
	case ChartQuadrant:
		plotQuadrant(cells, o, minY, maxY, plotW, plotH, n)
	case ChartGraph:
		plotGraph(cells, o, minY, maxY, plotW, plotH, n)
	default:
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
			hasLine := cell.bits != 0 || cell.r != 0 || cell.halves != 0
			if !ok || (!hasLine && !cell.fill) {
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
			cellColor := cell.color
			if !hasLine && cell.fill {
				r = o.FillGlyph
				if r == 0 {
					r = '░'
				}
				cellColor = cell.fillColor
			} else {
				switch o.Style {
				case ChartBraille:
					r = rune(0x2800) + rune(cell.bits)
				case ChartBox:
					r = boxGlyph(cell.bits)
				case ChartASCII:
					r = asciiGlyph(cell.bits)
				case ChartQuadrant:
					r = quadrantGlyph(cell.bits)
				case ChartGraph:
					// r already holds the ntcharts glyph
				default:
					if r == 0 {
						r = halfBlock(cell.halves)
					}
				}
			}
			if cellColor != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(cellColor).Render(string(r)))
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

// Box drawing direction bits. Bits point from a cell toward its neighbours.
const (
	boxN uint16 = 1 << iota
	boxS
	boxE
	boxW
	boxNE
	boxNW
	boxSE
	boxSW
	boxDot
)

// plotBox draws connected box-drawing lines. Each column gets the average of
// the samples that fall in it, then adjacent columns are joined by a line.
func plotBox(cells map[[2]int]chartCell, o LineOptions, minY, maxY float64, plotW, plotH, n int) {
	add := func(x, y int, bit uint16, c color.Color, prio int) {
		if x < 0 || y < 0 || x >= plotW || y >= plotH {
			return
		}
		key := [2]int{x, y}
		existing, ok := cells[key]
		if !ok {
			cells[key] = chartCell{bits: bit, color: c, priority: prio}
			return
		}
		if existing.priority != prio {
			return // first series owns this cell
		}
		existing.bits |= bit
		cells[key] = existing
	}

	for si, s := range o.Series {
		sums := make([]float64, plotW)
		counts := make([]int, plotW)
		for i, v := range s.Values {
			if v == nil {
				continue
			}
			col := solidXFor(i, n, plotW)
			sums[col] += *v
			counts[col]++
		}
		prevCol, prevY := -10, 0
		for col := 0; col < plotW; col++ {
			if counts[col] == 0 {
				prevCol = -10
				continue
			}
			value := sums[col] / float64(counts[col])
			y := cellYFor(value, minY, maxY, plotH)
			if prevCol == col-1 {
				drawBoxSegment(add, prevCol, prevY, col, y, s.Color, si)
			} else {
				add(col, y, boxDot, s.Color, si)
			}
			prevCol, prevY = col, y
		}
	}
}

func drawBoxSegment(add func(int, int, uint16, color.Color, int), x0, y0, x1, y1 int, c color.Color, prio int) {
	dx := x1 - x0
	dy := y1 - y0
	adx := absInt(dx)
	ady := absInt(dy)
	sx := 1
	if dx < 0 {
		sx = -1
	}
	sy := 1
	if dy < 0 {
		sy = -1
	}
	err := adx - ady
	for x0 != x1 || y0 != y1 {
		e2 := 2 * err
		nx, ny := x0, y0
		if e2 > -ady {
			err -= ady
			nx += sx
		}
		if e2 < adx {
			err += adx
			ny += sy
		}
		forward, back := boxDirection(x0, y0, nx, ny)
		add(x0, y0, forward, c, prio)
		add(nx, ny, back, c, prio)
		x0, y0 = nx, ny
	}
}

func boxDirection(x0, y0, x1, y1 int) (forward, back uint16) {
	switch {
	case x1 > x0 && y1 == y0:
		return boxE, boxW
	case x1 < x0 && y1 == y0:
		return boxW, boxE
	case x1 == x0 && y1 < y0:
		return boxN, boxS
	case x1 == x0 && y1 > y0:
		return boxS, boxN
	case x1 > x0 && y1 < y0:
		return boxNE, boxSW
	case x1 > x0 && y1 > y0:
		return boxSE, boxNW
	case x1 < x0 && y1 < y0:
		return boxNW, boxSE
	default:
		return boxSW, boxNE
	}
}

// boxGlyph maps a set of connection bits to the right box-drawing rune.
func boxGlyph(bits uint16) rune {
	if bits == 0 {
		return ' '
	}
	n := bits&boxN != 0
	s := bits&boxS != 0
	e := bits&boxE != 0
	w := bits&boxW != 0
	if !n && !s && !e && !w {
		switch {
		case bits&(boxNE|boxSW) != 0:
			return '╱'
		case bits&(boxNW|boxSE) != 0:
			return '╲'
		case bits&boxDot != 0:
			return '•'
		default:
			return '•'
		}
	}
	switch {
	case n && s && e && w:
		return '┼'
	case n && s && e:
		return '├'
	case n && s && w:
		return '┤'
	case n && e && w:
		return '┴'
	case s && e && w:
		return '┬'
	case n && s:
		return '│'
	case e && w:
		return '─'
	case n && e:
		return '└'
	case n && w:
		return '┘'
	case s && e:
		return '┌'
	case s && w:
		return '┐'
	case n, s:
		return '│'
	default:
		return '─'
	}
}

// cellYFor maps a value to a cell row.
func cellYFor(v, minY, maxY float64, plotH int) int {
	if maxY <= minY {
		return 0
	}
	return int(math.Round((maxY - v) / (maxY - minY) * float64(plotH-1)))
}

// plotAreaFill shades the cells below one series so the generation shape is
// readable even before the line is dense.
func plotAreaFill(cells map[[2]int]chartCell, o LineOptions, minY, maxY float64, plotW, plotH, n int) {
	idx := o.FillSeries
	if idx < 0 || idx >= len(o.Series) {
		return
	}
	s := o.Series[idx]
	sums := make([]float64, plotW)
	counts := make([]int, plotW)
	for i, v := range s.Values {
		if v == nil {
			continue
		}
		col := solidXFor(i, n, plotW)
		sums[col] += *v
		counts[col]++
	}
	for col := 0; col < plotW; col++ {
		if counts[col] == 0 {
			continue
		}
		row := cellYFor(sums[col]/float64(counts[col]), minY, maxY, plotH)
		baseline := cellYFor(0, minY, maxY, plotH)
		if baseline >= plotH {
			baseline = plotH - 1
		}
		if baseline < 0 {
			baseline = 0
		}
		for r := row + 1; r <= baseline; r++ {
			if r < 0 || r >= plotH {
				continue
			}
			key := [2]int{col, r}
			cell := cells[key]
			cell.fill = true
			cell.fillColor = s.Color
			cells[key] = cell
		}
	}
}

// plotQuadrant draws at 2x2 sub-cell resolution with quadrant blocks.
func plotQuadrant(cells map[[2]int]chartCell, o LineOptions, minY, maxY float64, plotW, plotH, n int) {
	pw := plotW * 2
	ph := plotH * 2
	set := func(px, py int, c color.Color, priority int) {
		if px < 0 || py < 0 || px >= pw || py >= ph {
			return
		}
		cx, cy := px/2, py/2
		key := [2]int{cx, cy}
		bit := uint16(1) << uint((py%2)*2+(px%2))
		existing, ok := cells[key]
		if !ok {
			cells[key] = chartCell{bits: bit, color: c, priority: priority}
			return
		}
		if priority < existing.priority {
			cells[key] = chartCell{bits: existing.bits | bit, fill: existing.fill, fillColor: existing.fillColor, color: c, priority: priority}
			return
		}
		existing.bits |= bit
		cells[key] = existing
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
				drawLine(set, prevX, prevY, px, py, s.Color, si)
			}
			set(px, py, s.Color, si)
			prevX, prevY = px, py
		}
	}
}

// quadrantGlyph maps four quadrant bits to a block rune.
func quadrantGlyph(bits uint16) rune {
	switch bits & 0x0F {
	case 1:
		return '▘'
	case 2:
		return '▝'
	case 3:
		return '▀'
	case 4:
		return '▖'
	case 5:
		return '▌'
	case 6:
		return '▞'
	case 7:
		return '▛'
	case 8:
		return '▗'
	case 9:
		return '▚'
	case 10:
		return '▐'
	case 11:
		return '▜'
	case 12:
		return '▄'
	case 13:
		return '▙'
	case 14:
		return '▟'
	case 15:
		return '█'
	default:
		return ' '
	}
}

// asciiGlyph maps box-drawing directions to their ASCII equivalents.
func asciiGlyph(bits uint16) rune {
	n := bits&boxN != 0
	s := bits&boxS != 0
	e := bits&boxE != 0
	w := bits&boxW != 0
	if !n && !s && !e && !w {
		if bits&boxDot != 0 {
			return '*'
		}
		return '/'
	}
	switch {
	case n && s && e && w:
		return '+'
	case n && e:
		return '+'
	case n && w:
		return '+'
	case s && e:
		return '+'
	case s && w:
		return '+'
	case (n || s) && (e || w):
		return '+'
	case n || s:
		return '|'
	default:
		return '-'
	}
}

// plotGraph uses ntcharts' canvas graph renderer for thin lines with corners.
func plotGraph(cells map[[2]int]chartCell, o LineOptions, minY, maxY float64, plotW, plotH, n int) {
	cv := ncanvas.New(plotW, plotH)
	for _, s := range o.Series {
		style := lipgloss.NewStyle().Foreground(s.Color)
		sums := make([]float64, plotW)
		counts := make([]int, plotW)
		for i, v := range s.Values {
			if v == nil {
				continue
			}
			col := solidXFor(i, n, plotW)
			sums[col] += *v
			counts[col]++
		}
		var pts []ncanvas.Point
		flush := func() {
			switch len(pts) {
			case 0:
			case 1:
				cv.SetRuneWithStyle(pts[0], '•', style)
			default:
				ngraph.DrawLinePoints(&cv, pts, nrunes.ArcLineStyle, style)
			}
			pts = pts[:0]
		}
		for col := 0; col < plotW; col++ {
			if counts[col] == 0 {
				flush()
				continue
			}
			row := cellYFor(sums[col]/float64(counts[col]), minY, maxY, plotH)
			pts = append(pts, ncanvas.Point{X: col, Y: row})
		}
		flush()
	}
	for y := 0; y < plotH; y++ {
		for x := 0; x < plotW; x++ {
			c := cv.Cell(ncanvas.Point{X: x, Y: y})
			if c.Rune == 0 || c.Rune == ' ' {
				continue
			}
			cells[[2]int{x, y}] = chartCell{r: c.Rune, color: c.Style.GetForeground()}
		}
	}
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
		bit := uint16(brailleBits[py%4][px%2])
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
