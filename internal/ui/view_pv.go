package ui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func (m *Model) renderPV(w, h, y int) string {
	var lines []string
	title := m.th.Bold(m.th.Accent).Render("PV STRINGS") + "  " + m.th.Muted.Render(m.snap.Model)
	lines = append(lines, title)

	total := 0.0
	var volts, amps float64
	voltsN := 0
	haveA := false
	for _, s := range m.snap.PVStrings {
		total += provider.Conv(s.Power)
		if s.Voltage != nil {
			volts += *s.Voltage
			voltsN++
		}
		if s.Current != nil {
			amps += *s.Current
			haveA = true
		}
	}
	summary := []string{
		m.th.Style(m.th.Solar).Render(fmtPowerAbs(total)),
		m.th.Muted.Render(itoaInt(len(m.snap.PVStrings)) + " str"),
		m.th.Muted.Render(fmtTempC(m.snap.TempC)),
	}
	if voltsN > 0 {
		avg := volts / float64(voltsN)
		summary = append(summary, m.th.Muted.Render(fmtVoltsC(&avg)))
	}
	if haveA {
		summary = append(summary, m.th.Muted.Render(fmtAmps(&amps)))
	}
	if total > 0 && m.snap.SolarW != nil {
		eff := *m.snap.SolarW / total * 100
		if eff > 0 && eff < 200 {
			summary = append(summary, m.th.Muted.Render("eff ")+m.th.Good.Render(fmtPct1(eff)))
		}
	}
	lines = append(lines, clampText(strings.Join(summary, "   "), w))

	if len(m.snap.PVStrings) == 0 {
		lines = append(lines, centerText(m.th.Muted.Render("no PV string data in the last snapshot"), w))
		return strings.Join(fitLines(lines, w, h), "\n")
	}

	header := m.th.Label.Render(padRight("#", 3) + padRight("VOLTS", 9) + padRight("AMPS", 8) + padRight("WATTS", 10) + "SHARE")
	lines = append(lines, header)

	visible := h - len(lines)
	if visible < 1 {
		visible = 1
	}
	count := len(m.snap.PVStrings)
	start := clamp(m.pvScroll, 0, max(0, count-visible))
	end := min(count, start+visible)
	barW := clamp(w-34, 8, 30)
	for i := start; i < end; i++ {
		s := m.snap.PVStrings[i]
		if s.Index == 0 {
			s.Index = i + 1
		}
		share := 0.0
		if total > 0 {
			share = provider.Conv(s.Power) / total
		}
		bar := shareBar(share, barW, m.th.Solar)
		row := padRight(itoaInt(s.Index), 3) +
			padRight(fmtVolts(s.Voltage), 9) +
			padRight(fmtAmps(s.Current), 8) +
			m.th.Style(m.th.Solar).Render(padRight(fmtPower(s.Power), 10)) +
			bar + m.th.Muted.Render(fmtPct1(share*100))
		lines = append(lines, row)
	}
	if end < count {
		lines = append(lines, m.th.Muted.Render(fmt.Sprintf("%d more…", count-end)))
	}
	return strings.Join(fitLines(lines, w, h), "\n")
}

func shareBar(frac float64, width int, c color.Color) string {
	if width <= 0 {
		return ""
	}
	filled := int(math.Round(frac * float64(width)))
	filled = clamp(filled, 0, width)
	var b strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			b.WriteString(lipgloss.NewStyle().Foreground(c).Render("█"))
		} else {
			b.WriteString(mutedBlock())
		}
	}
	return b.String()
}

func mutedBlock() string { return "░" }

func fmtPct1(v float64) string {
	return fmtFixed(v, 1) + "%"
}

func itoaInt(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
