package ui

import (
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

type flowBox struct {
	title      string
	value      string
	sub        string
	width      int
	border     color.Color
	titleColor color.Color
	valueColor color.Color
}

// renderFlow draws the energy-flow diagram as exactly height lines. It falls
// back to a compact three-line summary on narrow terminals.
func renderFlow(w, h int, snap provider.Snapshot, th Theme) []string {
	if w < 66 || h < 8 {
		return renderFlowCompact(w, h, snap, th)
	}
	boxW := 22
	pvW := 24
	gap := 3
	total := boxW*3 + gap*2
	left := (w - total) / 2
	if left < 0 {
		left = 0
	}

	grid := flowBox{
		title: "GRID", width: boxW, border: th.Faint, titleColor: th.Grid,
		valueColor: th.Grid, value: fmtPower(snap.GridW), sub: gridDirection(snap),
	}
	house := flowBox{
		title: "HOUSE", width: boxW, border: th.Faint, titleColor: th.Green,
		valueColor: th.Fg, value: fmtPower(snap.LoadW), sub: "bkp " + fmtPower(snap.BackupW),
	}
	battery := flowBox{
		title: "BATTERY", width: boxW, border: th.Faint, titleColor: th.Battery,
		valueColor: th.Battery, value: fmtPct(snap.SOC), sub: batteryDirection(snap),
	}
	pv := flowBox{
		title: "PV", width: pvW, border: th.Faint, titleColor: th.Solar,
		valueColor: th.Solar, value: fmtPower(snap.SolarW), sub: stringCountLabel(snap),
	}

	pvLines := boxLines(pv, th)
	gridLines := boxLines(grid, th)
	houseLines := boxLines(house, th)
	batteryLines := boxLines(battery, th)

	rows := make([]string, 0, 8)
	rows = append(rows, centerText(pvLines[0], w))
	rows = append(rows, centerText(pvLines[1], w))
	rows = append(rows, centerText(pvLines[2], w))

	houseCenter := left + boxW + gap + boxW/2
	rows = append(rows, markerLine(w, houseCenter, "▼", th.Solar))

	gapSpaces := strings.Repeat(" ", gap)
	rows = append(rows, strings.Repeat(" ", left)+gridLines[0]+gapSpaces+houseLines[0]+gapSpaces+batteryLines[0])

	rows = append(rows, strings.Repeat(" ", left)+gridLines[1]+gridConnector(snap, th, gap)+houseLines[1]+batteryConnector(snap, th, gap)+batteryLines[1])
	rows = append(rows, strings.Repeat(" ", left)+gridLines[2]+gapSpaces+houseLines[2]+gapSpaces+batteryLines[2])

	sinks := ""
	if snap.BackupW != nil || snap.LoadW != nil {
		smart := "—"
		if snap.LoadW != nil && snap.BackupW != nil {
			v := *snap.LoadW - *snap.BackupW
			if v < 0 {
				v = 0
			}
			smart = fmtPowerAbs(v)
		}
		sinks = "backup " + fmtPower(snap.BackupW) + "   ·   smart " + smart
	}
	if sinks != "" {
		rows = append(rows, centerText(th.Muted.Render(sinks), w))
	}
	return rows
}

func renderFlowCompact(w, h int, snap provider.Snapshot, th Theme) []string {
	gridDir := gridDirection(snap)
	line1 := th.Style(th.Solar).Render("PV "+fmtPower(snap.SolarW)) + th.Muted.Render(" ──┐")
	line2 := th.Style(th.Grid).Render("GRID "+fmtPower(snap.GridW)+" "+gridDir) + th.Muted.Render(" ──► ") +
		th.Style(th.Fg).Render("HOUSE "+fmtPower(snap.LoadW)) + th.Muted.Render(" ◄── ") +
		th.Style(th.Battery).Render("BATTERY "+fmtPct(snap.SOC)+" "+batteryDirection(snap))
	line3 := th.Muted.Render("backup " + fmtPower(snap.BackupW))
	out := []string{clampText(line1, w), clampText(line2, w), clampText(line3, w)}
	for len(out) < h {
		out = append(out, "")
	}
	return out[:max(0, min(h, len(out)))]
}

func boxLines(b flowBox, th Theme) [3]string {
	if b.width < 8 {
		b.width = 8
	}
	title := clampText(b.title, b.width-6)
	titleW := ansi.StringWidth(title)
	fill := b.width - 5 - titleW
	if fill < 0 {
		fill = 0
	}
	top := th.Rule.Render("╭─ ") + th.Bold(b.titleColor).Render(title) + th.Rule.Render(" "+strings.Repeat("─", fill)+"╮")
	contentW := b.width - 4
	value := clampText(b.value, contentW)
	sub := clampText(b.sub, max(0, contentW-ansi.StringWidth(value)-1))
	pad := contentW - ansi.StringWidth(value) - ansi.StringWidth(sub)
	if pad < 0 {
		pad = 0
	}
	mid := th.Rule.Render("│ ") +
		th.Bold(b.valueColor).Render(value) +
		strings.Repeat(" ", pad) +
		th.Muted.Render(sub) +
		th.Rule.Render(" │")
	bottom := th.Rule.Render("╰" + strings.Repeat("─", b.width-2) + "╯")
	return [3]string{top, mid, bottom}
}

func markerLine(w, at int, marker string, c color.Color) string {
	if at < 0 {
		at = 0
	}
	if at >= w {
		at = w - 1
	}
	return strings.Repeat(" ", at) + lipgloss.NewStyle().Foreground(c).Render(marker)
}

func gridDirection(snap provider.Snapshot) string {
	if snap.GridW == nil {
		return "—"
	}
	if *snap.GridW > 1 {
		return "EXPORT"
	}
	if *snap.GridW < -1 {
		return "IMPORT"
	}
	return "IDLE"
}

func batteryDirection(snap provider.Snapshot) string {
	if snap.BatteryW == nil || *snap.BatteryW == 0 {
		if snap.SOC != nil {
			return "idle"
		}
		return "—"
	}
	if *snap.BatteryW > 0 {
		return fmtPowerAbs(*snap.BatteryW) + " CHG"
	}
	return fmtPowerAbs(*snap.BatteryW) + " DIS"
}

func gridConnector(snap provider.Snapshot, th Theme, gap int) string {
	if gap <= 0 {
		return ""
	}
	dashes := strings.Repeat("─", max(0, gap-1))
	if snap.GridW == nil {
		return th.Muted.Render(dashes + " ")
	}
	if *snap.GridW > 1 {
		return th.Style(th.Export).Render("◄" + dashes)
	}
	if *snap.GridW < -1 {
		return th.Style(th.Import).Render(dashes + "►")
	}
	return th.Muted.Render(dashes + " ")
}

func batteryConnector(snap provider.Snapshot, th Theme, gap int) string {
	if gap <= 0 {
		return ""
	}
	dashes := strings.Repeat("─", max(0, gap-1))
	if snap.BatteryW == nil {
		return th.Muted.Render(" " + dashes)
	}
	if *snap.BatteryW > 1 {
		return th.Style(th.Battery).Render(dashes + "►")
	}
	if *snap.BatteryW < -1 {
		return th.Style(th.Discharge).Render("◄" + dashes)
	}
	return th.Muted.Render(" " + dashes)
}

func stringCountLabel(snap provider.Snapshot) string {
	n := len(snap.PVStrings)
	if n == 0 {
		return ""
	}
	if n == 1 {
		return "1 string"
	}
	return strconv.Itoa(n) + " strings"
}
