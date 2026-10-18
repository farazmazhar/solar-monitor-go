package ui

import (
	"strings"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

func (m *Model) renderAlarms(w, h, y int) string {
	filters := []string{"active", "last 30 days", "all"}
	var lines []string
	title := m.th.Bold(m.th.Accent).Render("ALARMS") +
		m.th.Muted.Render("   filter: "+filters[m.alarmFilter]+"   (tab cycles)")
	lines = append(lines, title)

	rows := m.filteredAlarms()
	if len(rows) == 0 {
		msg := "no alarms in this filter"
		if m.alarmsErr != nil {
			msg = m.alarmsErr.Error()
		}
		lines = append(lines, centerText(m.th.Good.Render("✓ ")+m.th.Muted.Render(msg), w))
		return strings.Join(fitLines(lines, w, h), "\n")
	}

	header := m.th.Label.Render(
		padRight("LEVEL", 10) + padRight("START", 18) + padRight("CODE", 8) + padRight("MESSAGE", 34) + "STATE")
	lines = append(lines, header)

	detailH := 3
	visible := h - len(lines) - detailH
	if visible < 1 {
		visible = 1
	}
	start := clamp(m.alarmSel-visible/2, 0, max(0, len(rows)-visible))
	end := min(len(rows), start+visible)
	for i := start; i < end; i++ {
		a := rows[i]
		line := alarmLevelCell(m.th, a) +
			padRight(m.th.Muted.Render(a.Begin.Format("2006-01-02 15:04")), 18) +
			padRight(m.th.Style(m.th.Fg).Render(a.Code), 8) +
			padRight(clampText(a.Message, 34), 34) +
			alarmStateCell(m.th, a)
		if i == m.alarmSel {
			line = m.th.Selected.Render(padRight(" "+line, w))
		}
		lines = append(lines, padRight(line, w))
		m.hits.Add(Rect{X: 0, Y: y + len(lines) - 1, W: w, H: 1}, Action{Kind: ActionAlarmRow, Row: i})
	}

	for len(lines) < h-detailH {
		lines = append(lines, "")
	}
	sel := rows[clamp(m.alarmSel, 0, len(rows)-1)]
	lines = append(lines, divider(m.th, w))
	lines = append(lines, m.th.Label.Render("advice: ")+m.th.Muted.Render(clampText(sel.Advice, w-9)))
	ended := "active"
	if !sel.End.IsZero() {
		ended = "ended " + sel.End.Format("2006-01-02 15:04")
	}
	lines = append(lines, m.th.Muted.Render("code "+sel.Code+" · began "+sel.Begin.Format("2006-01-02 15:04")+" · "+ended))
	return strings.Join(fitLines(lines, w, h), "\n")
}

func alarmLevelCell(th Theme, a provider.Alarm) string {
	label := "tip"
	c := th.Muted
	switch a.Level {
	case 2:
		label, c = "general", th.Warn
	case 3:
		label, c = "emergency", th.Bad
	}
	if a.Active {
		label = "!" + label
	}
	return c.Render(padRight("● "+label, 10))
}

func alarmStateCell(th Theme, a provider.Alarm) string {
	state := a.State
	if a.Active {
		return th.Bad.Render("ACTIVE")
	}
	if state == "" {
		state = "restored"
	}
	return th.Muted.Render(state)
}
