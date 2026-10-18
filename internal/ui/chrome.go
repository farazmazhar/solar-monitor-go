package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func fitFrame(s string, w, h int) string {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	for i, l := range lines {
		lines[i] = padRight(l, w)
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderHeader(w int) string {
	left := m.th.AppName.Render("solar-monitor-go")
	station := m.snap.Station
	if station == "" {
		station = "SolisCloud"
	}
	model := m.snap.Model
	meta := station
	if model != "" {
		meta += " · " + model
	}
	meta = clampText(meta, max(0, w-42))

	var right string
	switch {
	case m.setupErr != nil:
		right = m.th.Pill("SETUP NEEDED", m.th.Warning)
	default:
		state := m.th.Muted.Render("○ WAITING")
		switch {
		case m.demo:
			state = m.th.Pill("DEMO", m.th.Accent)
		case m.haveSnap && m.snap.State == provider.StateOnline:
			state = m.th.Pill("● ONLINE", m.th.Green)
		case m.haveSnap && m.snap.State == provider.StateAlarm:
			state = m.th.Pill("● ALARM", m.th.Danger)
		case m.haveSnap && m.snap.State == provider.StateOffline:
			state = m.th.Pill("● OFFLINE", m.th.Danger)
		case m.haveSnap:
			state = m.th.Pill("● UNKNOWN", m.th.Dim)
		}
		age := "—"
		switch {
		case m.demo:
			age = "simulated"
		case m.haveSnap:
			if a := fmtAgeFrom(m.now(), m.snap.FetchedAt); a == "now" {
				age = "just now"
			} else {
				age = a + " ago"
			}
		}
		ageStyle := m.th.Muted
		if m.haveSnap && time.Since(m.snap.FetchedAt) > 2*m.cfg.PollIntervalDuration() {
			ageStyle = m.th.Warn
		}
		alarms := ""
		if n := m.activeAlarmCount(); n > 0 {
			alarms = "  " + m.th.Bad.Render(fmt.Sprintf("⚠ %d", n))
		}
		right = state + "  " + ageStyle.Render(age) + "  " + m.th.Muted.Render(fmtClock(m.now())) + alarms
	}
	return joinSides(left+"  "+m.th.HeaderMeta.Render(meta), right, w)
}

func (m *Model) renderTabs(w, y int) string {
	var b strings.Builder
	x := 0
	for i, title := range tabTitles {
		style := m.th.TabInactive
		if Tab(i) == m.tab {
			style = m.th.TabActive
		}
		rendered := style.Render(" " + title + " ")
		m.hits.Add(Rect{X: x, Y: y, W: ansi.StringWidth(rendered), H: 1}, Action{Kind: ActionTab, Tab: Tab(i)})
		b.WriteString(rendered)
		x += ansi.StringWidth(rendered)
	}
	right := m.th.Muted.Render(m.now().Format("Mon 2 Jan"))
	return joinSides(b.String(), right, w)
}

func (m *Model) renderFooter(w int) string {
	left := m.footerKeys()
	var right string
	switch {
	case m.exportErr != nil:
		right = m.th.Bad.Render(clampText(m.exportErr.Error(), w/2))
	case m.exportMsg != "":
		right = m.th.Good.Render(clampText(m.exportMsg, w/2))
	case m.notice != "":
		right = m.th.Warn.Render(clampText(m.notice, w/2))
	default:
		if err := m.errorText(); err != "" {
			right = m.th.Bad.Render(clampText(err, w/2))
			break
		}
		next := ""
		if !m.nextPoll.IsZero() {
			next = "next " + fmtCountdown(m.now(), m.nextPoll)
		}
		right = m.th.Muted.Render(next)
	}
	return joinSides(left, right, w)
}

func (m *Model) footerKeys() string {
	pairs := [][2]string{{"1-7", "views"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	switch m.tab {
	case TabLive:
		pairs = [][2]string{{"t", "today"}, {"x", "window"}, {"m", "motion"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	case TabToday:
		pairs = [][2]string{{"→", "cursor"}, {"s l g b", "series"}, {"t", "today"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	case TabHistory:
		pairs = [][2]string{{"tab", "day/month/year"}, {"h l", "period"}, {"t", "now"}, {"e", "export"}, {"?", "help"}, {"q", "quit"}}
	case TabBattery:
		pairs = [][2]string{{"→", "cursor"}, {"t", "today"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	case TabPV:
		pairs = [][2]string{{"j k", "scroll"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	case TabAlarms:
		pairs = [][2]string{{"tab", "filter"}, {"j k", "select"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	case TabDiagnostics:
		pairs = [][2]string{{"j k", "scroll"}, {"e", "export"}, {"d", "re-discover"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	}
	var parts []string
	for _, p := range pairs {
		parts = append(parts, m.th.HelpKey.Render(p[0])+" "+m.th.HelpDesc.Render(p[1]))
	}
	return strings.Join(parts, m.th.HelpDesc.Render("  ·  "))
}

func (m *Model) renderScreen(w, h, y int) string {
	switch m.tab {
	case TabLive:
		return m.renderLive(w, h, y)
	case TabToday:
		return m.renderToday(w, h, y)
	case TabHistory:
		return m.renderHistory(w, h, y)
	case TabBattery:
		return m.renderBattery(w, h, y)
	case TabPV:
		return m.renderPV(w, h, y)
	case TabAlarms:
		return m.renderAlarms(w, h, y)
	case TabDiagnostics:
		return m.renderDiagnostics(w, h, y)
	}
	return strings.Repeat("\n", h)
}

func (m *Model) activeAlarmCount() int {
	n := 0
	for _, a := range m.alarms {
		if a.Active {
			n++
		}
	}
	return n
}

func (m *Model) renderSetup(w, h int) string {
	lines := []string{
		m.th.Bold(m.th.Warning).Render("Nothing to monitor yet"),
		"",
		"The SolisCloud API needs credentials before live data can be shown.",
		"",
		m.th.Label.Render("Config file: ") + m.paths.ConfigPath,
		m.th.Label.Render("Required keys: ") + "solis.api_key, solis.api_secret, solis.station_id",
		"",
		m.th.HelpKey.Render("solar-monitor --init") + m.th.HelpDesc.Render("  writes a template config"),
		m.th.HelpKey.Render("d") + m.th.HelpDesc.Render("  start the built-in demo now"),
		m.th.HelpKey.Render("?") + m.th.HelpDesc.Render("  help"),
		m.th.HelpKey.Render("q") + m.th.HelpDesc.Render("  quit"),
	}
	if m.setupErr != nil {
		lines = append([]string{m.th.Bad.Render(m.setupErr.Error()), ""}, lines...)
	}
	return fitFrame(placeCenter(strings.Join(lines, "\n"), w, h), w, h)
}

func (m *Model) renderHelp(w, h int) string {
	rows := [][2]string{
		{"1-7", "switch screen"},
		{"tab / shift+tab", "next / previous screen, or cycle filter"},
		{"← →", "move the chart cursor"},
		{"j k / ↑ ↓", "move in tables and scroll raw data"},
		{"r", "refresh now"},
		{"m", "toggle motion"},
		{"x", "toggle fixed-day / fit-samples chart window"},
		{"t", "jump to today / current period"},
		{"s l g b", "toggle Today series (solar, load, grid, battery)"},
		{"h l", "previous / next period in History"},
		{"e", "export JSON or CSV"},
		{"d", "re-discover the inverter (Diagnostics only)"},
		{"?", "close this help"},
		{"q", "quit"},
	}
	width := min(64, max(30, w-6))
	var lines []string
	lines = append(lines, m.th.Bold(m.th.Accent).Render("Keys"), "")
	for _, r := range rows {
		lines = append(lines, "  "+padRight(m.th.HelpKey.Render(r[0]), 18)+m.th.HelpDesc.Render(r[1]))
	}
	lines = append(lines, "", m.th.Muted.Render("Mouse: click tabs, tiles and chart; wheel scrolls lists."))
	box := renderBox(m.th, "help", lines, width)
	return fitFrame(placeCenter(box, w, h), w, h)
}

func renderBox(th Theme, title string, lines []string, width int) string {
	if width < 8 {
		width = 8
	}
	inner := width - 4
	var b strings.Builder
	if title != "" {
		t := clampText(title, inner)
		fill := inner - ansi.StringWidth(t)
		b.WriteString(th.Rule.Render("╭─ ") + th.Bold(th.Accent).Render(t) + th.Rule.Render(" "+strings.Repeat("─", max(0, fill-1))+"╮"))
	} else {
		b.WriteString(th.Rule.Render("╭" + strings.Repeat("─", width-2) + "╮"))
	}
	for _, line := range lines {
		b.WriteString("\n")
		b.WriteString(th.Rule.Render("│ ") + padRight(line, inner) + th.Rule.Render(" │"))
	}
	b.WriteString("\n")
	b.WriteString(th.Rule.Render("╰" + strings.Repeat("─", width-2) + "╯"))
	return b.String()
}

func placeCenter(block string, w, h int) string {
	lines := strings.Split(block, "\n")
	blockW := 0
	for _, l := range lines {
		blockW = max(blockW, ansi.StringWidth(l))
	}
	top := max(0, (h-len(lines))/2)
	var out []string
	for i := 0; i < top; i++ {
		out = append(out, strings.Repeat(" ", w))
	}
	left := max(0, (w-blockW)/2)
	for _, l := range lines {
		out = append(out, strings.Repeat(" ", left)+padRight(l, blockW)+strings.Repeat(" ", max(0, w-left-blockW)))
	}
	for len(out) < h {
		out = append(out, strings.Repeat(" ", w))
	}
	return strings.Join(out, "\n")
}

// divider renders a dim full-width rule.
func divider(th Theme, w int) string {
	return th.Rule.Render(strings.Repeat("─", max(0, w)))
}
