package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

type dayStats struct {
	generated, consumed, imported, exported float64
	charged, discharged, peak               float64
	peakAt                                  time.Time
}

func computeDayStats(samples []provider.Sample) dayStats {
	var st dayStats
	step := 5.0 / 60.0 // kWh per five-minute sample at one kW
	for _, s := range samples {
		st.generated += provider.Conv(s.SolarW) / 1000 * step
		st.consumed += provider.Conv(s.LoadW) / 1000 * step
		st.imported += provider.Conv(s.ImportW) / 1000 * step
		st.exported += provider.Conv(s.ExportW) / 1000 * step
		if s.BatteryW != nil {
			if *s.BatteryW > 0 {
				st.charged += *s.BatteryW / 1000 * step
			} else {
				st.discharged += -*s.BatteryW / 1000 * step
			}
		}
		if s.SolarW != nil && *s.SolarW > st.peak {
			st.peak = *s.SolarW
			st.peakAt = s.TS
		}
	}
	return st
}

func (m *Model) renderHistory(w, h, y int) string {
	title := m.th.Bold(m.th.Accent).Render("HISTORY")
	switch m.historyMode {
	case HistoryDay:
		title += "  " + m.th.Muted.Render("DAY") + "  " + m.th.Value.Render(fmtDate(m.historyDay))
	case HistoryMonth:
		title += "  " + m.th.Muted.Render("MONTH") + "  " + m.th.Value.Render(fmtMonth(m.monthAt))
	case HistoryYear:
		title += "  " + m.th.Muted.Render("YEAR") + "  " + m.th.Value.Render(strconv.Itoa(m.yearAt.Year()))
	}
	title = joinSides(title, m.th.Muted.Render("tab mode · h l period · t now"), w)

	var lines []string
	lines = append(lines, title, "")

	switch m.historyMode {
	case HistoryDay:
		lines = append(lines, m.renderHistoryDay(w, h-len(lines), y+len(lines))...)
	case HistoryMonth, HistoryYear:
		lines = append(lines, m.renderHistoryPeriod(w, h-len(lines))...)
	}
	return strings.Join(fitLines(lines, w, h), "\n")
}

func (m *Model) renderHistoryDay(w, h, y int) []string {
	samples := m.historySamples
	st := computeDayStats(samples)
	summary := []string{
		"  " + m.th.Muted.Render("generated ") + m.th.Style(m.th.Solar).Render(fmtEnergyAbs(st.generated)) +
			m.th.Muted.Render("     consumed ") + m.th.Value.Render(fmtEnergyAbs(st.consumed)),
		"  " + m.th.Muted.Render("imported ") + m.th.Style(m.th.Import).Render(fmtEnergyAbs(st.imported)) +
			m.th.Muted.Render("     exported ") + m.th.Style(m.th.Export).Render(fmtEnergyAbs(st.exported)) +
			m.th.Muted.Render("     battery +") + m.th.Style(m.th.Battery).Render(fmtEnergyAbs(st.charged)) +
			m.th.Muted.Render(" / -") + m.th.Style(m.th.Discharge).Render(fmtEnergyAbs(st.discharged)),
		"  " + m.th.Muted.Render("peak ") + m.th.Style(m.th.Solar).Render(fmtPowerAbs(st.peak)) +
			m.th.Muted.Render(" at "+st.peakAt.Format("15:04")) +
			m.th.Muted.Render("     samples ") + m.th.Value.Render(strconv.Itoa(len(samples))),
	}
	lines := wrapPanel("SUMMARY", summary, w, m.th.Faint, m.th.Dim, m.th.Faint)
	lines = append(lines, "")
	lines = append(lines, m.cursorLine(w))

	chartH := h - len(lines)
	if chartH < 6 {
		chartH = 6
	}
	chartY := y + len(lines)
	chart, hit := m.renderDayChart(w, chartH, "POWER · "+fmtDate(m.historyDay), samples, m.historyDay, [4]bool{true, true, true, true}, true, true)
	lines = append(lines, chart...)
	m.hits.Add(
		Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H},
		Action{Kind: ActionChartCursor, Rect: Rect{X: hit.X, Y: chartY + hit.Y, W: hit.W, H: hit.H}},
	)
	return lines
}

func (m *Model) renderHistoryPeriod(w, h int) []string {
	gen, cons, imp, exp, income, charged, discharged := m.historyTotals()
	summary := []string{
		"  " + m.th.Muted.Render("generated ") + m.th.Style(m.th.Solar).Render(fmtEnergy(gen)) +
			m.th.Muted.Render("     consumed ") + m.th.Value.Render(fmtEnergy(cons)),
		"  " + m.th.Muted.Render("imported ") + m.th.Style(m.th.Import).Render(fmtEnergy(imp)) +
			m.th.Muted.Render("     exported ") + m.th.Style(m.th.Export).Render(fmtEnergy(exp)) +
			m.th.Muted.Render("     battery +") + m.th.Style(m.th.Battery).Render(fmtEnergy(charged)) +
			m.th.Muted.Render(" / -") + m.th.Style(m.th.Discharge).Render(fmtEnergy(discharged)),
		"  " + m.th.Muted.Render("income ") + m.th.Good.Render(fmtMoney(m.currencySymbol(), income)) +
			m.th.Muted.Render("     CO₂ ") + m.th.Good.Render(fmtCO2(kwhToCO2(gen, m.cfg.Display.CO2KgPerKWh))),
	}
	lines := wrapPanel("SUMMARY", summary, w, m.th.Faint, m.th.Dim, m.th.Faint)
	lines = append(lines, "")

	bars := m.historyBars()
	chartH := h - len(lines)
	if chartH < 7 {
		chartH = 7
	}
	if len(bars) == 0 {
		msg := "no history for this period"
		if m.historyMode == HistoryMonth && m.monthErr != nil {
			msg = m.monthErr.Error()
		}
		if m.historyMode == HistoryYear && m.yearErr != nil {
			msg = m.yearErr.Error()
		}
		lines = append(lines, wrapPanel("GENERATED", []string{centerText(m.th.Muted.Render(msg), w-4)}, w, m.th.Faint, m.th.Dim, m.th.Faint)...)
		return lines
	}
	step := max(1, len(bars)/8)
	barsView := RenderBars(BarOptions{
		Width: w, Height: chartH,
		Title:       "GENERATED · " + m.periodLabel(),
		Border:      true,
		BorderColor: m.th.Faint,
		TitleColor:  m.th.Accent,
		Bars:        bars,
		Dim:         m.th.Faint,
		Fg:          m.th.Fg,
		ShowYAxis:   true,
		LabelStep:   step,
		YLabel:      func(v float64) string { return trimNum(v) + " kWh" },
	})
	lines = append(lines, barsView...)
	return lines
}

func (m *Model) periodLabel() string {
	if m.historyMode == HistoryYear {
		return strconv.Itoa(m.yearAt.Year())
	}
	return m.monthAt.Format("January 2006")
}

func (m *Model) historyTotals() (gen, cons, imp, exp, income, charged, discharged *float64) {
	add := func(dst **float64, v *float64) {
		if v == nil {
			return
		}
		if *dst == nil {
			*dst = provider.Float(0)
		}
		**dst += *v
	}
	if m.historyMode == HistoryMonth {
		for _, r := range m.month {
			add(&gen, r.GeneratedKWh)
			add(&cons, r.ConsumedKWh)
			add(&imp, r.ImportedKWh)
			add(&exp, r.ExportedKWh)
			add(&charged, r.ChargedKWh)
			add(&discharged, r.DischargedKWh)
			add(&income, m.rowIncome(r.Income, r.Currency, r.GeneratedKWh))
		}
	} else {
		for _, r := range m.year {
			add(&gen, r.GeneratedKWh)
			add(&cons, r.ConsumedKWh)
			add(&imp, r.ImportedKWh)
			add(&exp, r.ExportedKWh)
			add(&charged, r.ChargedKWh)
			add(&discharged, r.DischargedKWh)
			add(&income, m.rowIncome(r.Income, r.Currency, r.GeneratedKWh))
		}
	}
	return
}

func (m *Model) historyBars() []Bar {
	var bars []Bar
	if m.historyMode == HistoryMonth {
		today := m.now()
		for _, r := range m.month {
			if r.GeneratedKWh == nil {
				continue
			}
			bars = append(bars, Bar{
				Label:     strconv.Itoa(r.Date.Day()),
				Value:     *r.GeneratedKWh,
				Color:     m.th.Solar,
				Highlight: sameDay(r.Date, today),
			})
		}
	} else {
		now := m.now()
		for _, r := range m.year {
			if r.GeneratedKWh == nil {
				continue
			}
			bars = append(bars, Bar{
				Label:     r.Month.Format("Jan"),
				Value:     *r.GeneratedKWh,
				Color:     m.th.Solar,
				Highlight: r.Month.Month() == now.Month() && r.Month.Year() == now.Year(),
			})
		}
	}
	return bars
}

func (m *Model) rowIncome(api *float64, currency string, generated *float64) *float64 {
	if api != nil && strings.EqualFold(currency, m.cfg.Display.Currency) {
		return api
	}
	if m.cfg.Tariff.Export == 0 || generated == nil {
		return nil
	}
	return provider.Float(*generated * m.cfg.Tariff.Export)
}

func kwhToCO2(kwh *float64, factor float64) *float64 {
	if kwh == nil || factor == 0 {
		return nil
	}
	return provider.Float(*kwh * factor)
}
