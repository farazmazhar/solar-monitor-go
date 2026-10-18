package ui

import (
	"strconv"
	"strings"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
)

func (m *Model) renderHistory(w, h, y int) string {
	var lines []string
	title := m.th.Bold(m.th.Accent).Render("HISTORY")
	if m.historyMode == HistoryMonth {
		title += "  " + m.th.Muted.Render(fmtMonth(m.monthAt))
	} else {
		title += "  " + m.th.Muted.Render(strconv.Itoa(m.yearAt.Year()))
	}
	title += m.th.Muted.Render("   tab toggles month/year   h l steps period")
	lines = append(lines, title)

	gen, cons, imp, exp, income, charged, discharged := m.historyTotals()
	stats := []string{
		m.th.Muted.Render("generated ") + m.th.Style(m.th.Solar).Render(fmtEnergy(gen)),
		m.th.Muted.Render("consumed ") + m.th.Style(m.th.Fg).Render(fmtEnergy(cons)),
		m.th.Muted.Render("imported ") + m.th.Style(m.th.Import).Render(fmtEnergy(imp)),
		m.th.Muted.Render("exported ") + m.th.Style(m.th.Export).Render(fmtEnergy(exp)),
	}
	lines = append(lines, clampText(strings.Join(stats, "   "), w))
	stats2 := []string{
		m.th.Muted.Render("battery +") + m.th.Style(m.th.Battery).Render(fmtEnergy(charged)),
		m.th.Muted.Render("− ") + m.th.Style(m.th.Discharge).Render(fmtEnergy(discharged)),
		m.th.Muted.Render("income ") + m.th.Good.Render(fmtMoney(m.currencySymbol(), income)),
		m.th.Muted.Render("CO₂ ") + m.th.Good.Render(fmtCO2(kwhToCO2(gen, m.cfg.Display.CO2KgPerKWh))),
	}
	lines = append(lines, clampText(strings.Join(stats2, "   "), w))

	bars := m.historyBars()
	if len(bars) == 0 {
		msg := "no history for this period"
		if m.historyMode == HistoryMonth && m.monthErr != nil {
			msg = m.monthErr.Error()
		}
		if m.historyMode == HistoryYear && m.yearErr != nil {
			msg = m.yearErr.Error()
		}
		lines = append(lines, centerText(m.th.Muted.Render(msg), w))
		return strings.Join(fitLines(lines, w, h), "\n")
	}
	chartH := h - len(lines)
	if chartH < 3 {
		chartH = 3
	}
	step := max(1, len(bars)/8)
	chart := RenderBars(BarOptions{
		Width: w, Height: chartH, Bars: bars,
		Dim: m.th.Faint, Fg: m.th.Fg, ShowYAxis: true, LabelStep: step,
	})
	lines = append(lines, chart...)
	return strings.Join(fitLines(lines, w, h), "\n")
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
