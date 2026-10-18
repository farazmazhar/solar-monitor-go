package ui

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"time"
)

func (m *Model) renderDiagnostics(w, h, y int) string {
	var lines []string
	title := m.th.Bold(m.th.Accent).Render("DIAGNOSTICS") +
		m.th.Muted.Render("   j k scroll raw · e export snapshot")
	lines = append(lines, title)

	source := "solis"
	if m.demo {
		source = "demo (in-memory)"
	}
	state := "waiting"
	if m.haveSnap {
		state = string(m.snap.State)
	}
	fetch := "—"
	if m.haveSnap {
		age := fmtAge(m.snap.FetchedAt)
		if age != "now" {
			age += " ago"
		}
		fetch = m.snap.FetchedAt.Format("2006-01-02 15:04:05") + " (" + age + ")"
	}
	latency := "—"
	if m.lastLatency > 0 {
		latency = m.lastLatency.Round(time.Millisecond).String()
	}
	db := m.paths.DBPath
	if m.demo {
		db = "in-memory (demo)"
	}
	errText := "none"
	if m.lastErr != nil {
		errText = m.lastErr.Error()
	}
	rows := [][2]string{
		{"source", source},
		{"api", m.cfg.Solis.BaseURL},
		{"station", m.snap.Station},
		{"inverter", m.snap.SN},
		{"state", state},
		{"last fetch", fetch},
		{"latency", latency},
		{"next poll", m.nextPoll.Format("15:04:05")},
		{"last error", errText},
		{"config", m.paths.ConfigPath},
		{"database", db},
		{"memory samples", itoaInt(len(m.daySamples))},
		{"version", m.version + " · " + runtime.Version()},
		{"terminal", itoaInt(m.width) + "x" + itoaInt(m.height) + " · theme " + m.th.Name},
	}
	for _, r := range rows {
		lines = append(lines, kv(r[0], m.th.Value.Render(clampText(r[1], w-18)), 16, m.th))
	}

	raw := m.rawJSONLines()
	if len(raw) > 0 {
		lines = append(lines, divider(m.th, w))
		visible := h - len(lines)
		if visible < 1 {
			visible = 1
		}
		maxScroll := max(0, len(raw)-visible)
		m.rawScroll = clamp(m.rawScroll, 0, maxScroll)
		header := m.th.Label.Render("RAW SNAPSHOT") + m.th.Muted.Render("  lines "+itoaInt(m.rawScroll+1)+"-"+itoaInt(min(len(raw), m.rawScroll+visible))+" of "+itoaInt(len(raw)))
		lines = append(lines, header)
		for i := m.rawScroll; i < len(raw) && len(lines) < h; i++ {
			lines = append(lines, m.th.Muted.Render(clampText(raw[i], w)))
		}
	} else if m.haveSnap {
		lines = append(lines, m.th.Muted.Render("raw payload not retained for this source"))
	}
	return strings.Join(fitLines(lines, w, h), "\n")
}

func (m *Model) rawJSONLines() []string {
	if len(m.snap.Raw) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, m.snap.Raw, "", "  "); err != nil {
		return strings.Split(string(m.snap.Raw), "\n")
	}
	return strings.Split(buf.String(), "\n")
}
