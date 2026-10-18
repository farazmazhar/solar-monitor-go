package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// fmtPower renders a nullable watt reading with automatic units.
func fmtPower(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmtPowerAbs(*p)
}

func fmtPowerAbs(w float64) string {
	a := math.Abs(w)
	switch {
	case a >= 1_000_000:
		return fmt.Sprintf("%.2f MW", a/1_000_000)
	case a >= 1000:
		return fmt.Sprintf("%.2f kW", a/1000)
	default:
		return fmt.Sprintf("%.0f W", a)
	}
}

func fmtPowerSigned(p *float64) string {
	if p == nil {
		return "—"
	}
	if *p >= 0 {
		return fmtPowerAbs(*p)
	}
	return fmtPowerAbs(*p)
}

func fmtEnergy(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmtEnergyAbs(*p)
}

func fmtEnergyAbs(kwh float64) string {
	a := math.Abs(kwh)
	if a >= 1000 {
		return fmt.Sprintf("%.2f MWh", a/1000)
	}
	if a >= 100 {
		return fmt.Sprintf("%.0f kWh", a)
	}
	return fmt.Sprintf("%.1f kWh", a)
}

// shortEnergy renders a compact kWh figure without the unit, for tight tiles.
func shortEnergy(p *float64) string {
	if p == nil {
		return "—"
	}
	a := math.Abs(*p)
	if a >= 1000 {
		return fmt.Sprintf("%.1fM", *p/1000)
	}
	if a >= 100 {
		return fmt.Sprintf("%.0f", *p)
	}
	return fmt.Sprintf("%.1f", *p)
}

func fmtPct(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", *p)
}

func fmtTemp(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.1f °C", *p)
}

// fmtTempC is the compact temperature form for tight summary rows.
func fmtTempC(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f°C", *p)
}

func fmtVolts(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.1f V", *p)
}

// fmtVoltsC is the compact voltage form for tight summary rows.
func fmtVoltsC(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.0f V", *p)
}

func fmtAmps(p *float64) string {
	if p == nil {
		return "—"
	}
	return fmt.Sprintf("%.1f A", *p)
}

func fmtMoney(sym string, p *float64) string {
	if p == nil || math.IsNaN(*p) || math.IsInf(*p, 0) {
		return "—"
	}
	return sym + " " + fmtThousands(*p)
}

func fmtThousands(v float64) string {
	neg := v < 0
	s := fmt.Sprintf("%.2f", math.Abs(v))
	whole, frac, _ := strings.Cut(s, ".")
	out := addCommas(whole) + "." + frac
	if neg {
		return "-" + out
	}
	return out
}

func addCommas(s string) string {
	if len(s) <= 3 {
		return s
	}
	var out []byte
	pre := len(s) % 3
	if pre > 0 {
		out = append(out, s[:pre]...)
		if len(s) > pre {
			out = append(out, ',')
		}
	}
	for i := pre; i < len(s); i += 3 {
		out = append(out, s[i:i+3]...)
		if i+3 < len(s) {
			out = append(out, ',')
		}
	}
	return string(out)
}

func fmtAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < 2*time.Second:
		return "now"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func fmtClock(t time.Time) string {
	if t.IsZero() {
		return "--:--"
	}
	return t.Format("15:04")
}

// fmtCountdown renders time until t, or its age when t is in the past.
func fmtCountdown(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Until(t)
	if d <= 0 {
		return fmtAge(t)
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("in %ds", int(d.Seconds()+0.5))
	case d < time.Hour:
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("in %dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Mon 2 Jan 2006")
}

func fmtMonth(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("January 2006")
}

func clampText(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "…")
}

func padRight(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw >= w {
		return clampText(s, w)
	}
	return s + strings.Repeat(" ", w-sw)
}

func padLeft(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw >= w {
		return clampText(s, w)
	}
	return strings.Repeat(" ", w-sw) + s
}

func centerText(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw >= w {
		return clampText(s, w)
	}
	left := (w - sw) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", w-sw-left)
}

func joinSides(left, right string, w int) string {
	lw := ansi.StringWidth(left)
	rw := ansi.StringWidth(right)
	if lw+rw+1 > w {
		right = clampText(right, max(0, w-lw-1))
		rw = ansi.StringWidth(right)
	}
	gap := w - lw - rw
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
