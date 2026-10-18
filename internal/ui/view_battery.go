package ui

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func (m *Model) renderBattery(w, h, y int) string {
	var lines []string
	title := m.th.Bold(m.th.Accent).Render("BATTERY") + "  " + m.th.Muted.Render(m.snap.Model)
	lines = append(lines, title)

	direction := m.th.Muted.Render("no battery data")
	switch {
	case m.snap.BatteryW == nil:
	case *m.snap.BatteryW > 1:
		direction = m.th.Style(m.th.Battery).Render(fmtPowerAbs(*m.snap.BatteryW) + " charging")
	case *m.snap.BatteryW < -1:
		direction = m.th.Style(m.th.Discharge).Render(fmtPowerAbs(*m.snap.BatteryW) + " discharging")
	default:
		direction = m.th.Muted.Render("idle")
	}
	lines = append(lines, m.renderSOCArc(w)...)
	lines = append(lines, "  "+direction+"   "+m.th.Muted.Render("SOH "+fmtPct(m.snap.SOH)))

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
	if chartH >= 5 && len(m.daySamples) > 0 {
		chartY := y + len(lines)
		soc := daySlotValues(m.daySamples, func(s provider.Sample) *float64 { return s.SOC })
		chart, hit := RenderLine(LineOptions{
			Width: w, Height: chartH,
			Title: "STATE OF CHARGE · FIXED DAY 00:00-23:59", Border: true,
			BorderColor: m.th.Faint, TitleColor: m.th.Accent,
			Style:  m.chartStyle(),
			Series: []LineSeries{{Name: "soc", Color: m.th.Battery, Values: soc}},
			Cursor: m.cursorSlot(m.day), CursorColor: m.th.Fg, Dim: m.th.Faint,
			ShowYAxis: true, ShowXAxis: true, ShowGrid: true, MinY: provider.Float(0), MaxY: provider.Float(100),
			XLabels: dayTimeLabels(m.day), YLabel: fmtPct1,
		})
		lines = append(lines, chart...)
		m.hits.Add(
			Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H},
			Action{Kind: ActionChartCursor, Rect: Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H}},
		)
	} else if len(m.daySamples) == 0 {
		lines = append(lines, m.th.Muted.Render("no SOC history yet"))
	}
	return strings.Join(fitLines(lines, w, h), "\n")
}

// renderSOCArc draws the state of charge as a half-donut with the percentage
// in the middle.
func (m *Model) renderSOCArc(w int) []string {
	rows := 5
	cols := min(w, 41)
	if cols < 20 {
		cols = 20
	}
	soc := 0.0
	have := false
	if m.snap.SOC != nil {
		soc = *m.snap.SOC
		have = true
	}
	gaugeColor := m.th.Battery
	if have {
		switch {
		case soc < 20:
			gaugeColor = m.th.Danger
		case soc < 50:
			gaugeColor = m.th.Warning
		default:
			gaugeColor = m.th.Green
		}
	}

	type cell struct {
		r rune
		c color.Color
	}
	grid := make([][]cell, rows)
	for y := range grid {
		grid[y] = make([]cell, cols)
	}
	cx := float64(cols-1) / 2
	cy := float64(rows - 1)
	rx := float64(cols-1) / 2
	ry := float64(rows - 1)
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			nx := (float64(x) - cx) / rx
			ny := (cy - float64(y)) / ry
			if ny < -0.1 {
				continue
			}
			if math.Abs(math.Hypot(nx, ny)-1) > 0.20 {
				continue
			}
			angle := math.Atan2(ny, nx)
			p := (math.Pi - angle) / math.Pi
			if p < 0 {
				p = 0
			}
			if p > 1 {
				p = 1
			}
			if have && p <= soc/100 {
				grid[y][x] = cell{r: '█', c: gaugeColor}
			} else {
				grid[y][x] = cell{r: '░', c: m.th.Faint}
			}
		}
	}

	pct := fmtPct(m.snap.SOC)
	textRow := rows / 2
	start := (cols - len(pct)) / 2
	for i, r := range pct {
		if start+i >= 0 && start+i < cols {
			grid[textRow][start+i] = cell{r: r, c: m.th.Fg}
		}
	}

	lines := make([]string, rows)
	for y := 0; y < rows; y++ {
		var b strings.Builder
		for x := 0; x < cols; x++ {
			cl := grid[y][x]
			if cl.r == 0 {
				b.WriteString(" ")
			} else if cl.c != nil {
				b.WriteString(lipgloss.NewStyle().Foreground(cl.c).Render(string(cl.r)))
			} else {
				b.WriteRune(cl.r)
			}
		}
		lines[y] = centerText(b.String(), w)
	}
	return lines
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
