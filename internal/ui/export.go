package ui

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) exportSnapshotCmd() tea.Cmd {
	if !m.haveSnap {
		return func() tea.Msg { return exportMsg{err: fmt.Errorf("nothing to export yet")} }
	}
	snap := m.snap
	dir := m.paths.StateDir
	return func() tea.Msg {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return exportMsg{err: err}
		}
		name := "snapshot-" + time.Now().Format("20060102-150405") + ".json"
		path := filepath.Join(dir, name)
		b, err := json.MarshalIndent(snap, "", "  ")
		if err != nil {
			return exportMsg{err: err}
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			return exportMsg{err: err}
		}
		return exportMsg{path: path}
	}
}

func (m *Model) exportHistoryCmd() tea.Cmd {
	dir := m.paths.StateDir
	mode := m.historyMode
	month := m.month
	year := m.year
	monthAt := m.monthAt
	yearAt := m.yearAt
	day := m.historyDay
	samples := m.historySamples
	return func() tea.Msg {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return exportMsg{err: err}
		}
		var (
			name   string
			header []string
			rows   [][]string
		)
		switch mode {
		case HistoryDay:
			name = "history-day-" + day.Format("2006-01-02") + ".csv"
			header = []string{"time", "solar_w", "load_w", "grid_w", "battery_w", "soc", "import_w", "export_w"}
			for _, s := range samples {
				rows = append(rows, []string{
					s.TS.Format("2006-01-02 15:04"),
					opt(s.SolarW), opt(s.LoadW), opt(s.GridW), opt(s.BatteryW), opt(s.SOC), opt(s.ImportW), opt(s.ExportW),
				})
			}
		case HistoryMonth:
			name = "history-month-" + monthAt.Format("2006-01") + ".csv"
			header = []string{"date", "generated_kwh", "consumed_kwh", "imported_kwh", "exported_kwh", "charged_kwh", "discharged_kwh", "income", "currency"}
			for _, r := range month {
				rows = append(rows, []string{
					r.Date.Format("2006-01-02"),
					opt(r.GeneratedKWh), opt(r.ConsumedKWh), opt(r.ImportedKWh), opt(r.ExportedKWh),
					opt(r.ChargedKWh), opt(r.DischargedKWh), opt(r.Income), r.Currency,
				})
			}
		case HistoryYear:
			name = "history-year-" + strconv.Itoa(yearAt.Year()) + ".csv"
			header = []string{"month", "generated_kwh", "consumed_kwh", "imported_kwh", "exported_kwh", "charged_kwh", "discharged_kwh", "income", "currency"}
			for _, r := range year {
				rows = append(rows, []string{
					r.Month.Format("2006-01"),
					opt(r.GeneratedKWh), opt(r.ConsumedKWh), opt(r.ImportedKWh), opt(r.ExportedKWh),
					opt(r.ChargedKWh), opt(r.DischargedKWh), opt(r.Income), r.Currency,
				})
			}
		}
		if len(rows) == 0 {
			return exportMsg{err: fmt.Errorf("no rows to export for this period")}
		}
		path := filepath.Join(dir, name)
		f, err := os.Create(path)
		if err != nil {
			return exportMsg{err: err}
		}
		defer f.Close()
		w := csv.NewWriter(f)
		if err := w.Write(header); err != nil {
			return exportMsg{err: err}
		}
		if err := w.WriteAll(rows); err != nil {
			return exportMsg{err: err}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return exportMsg{err: err}
		}
		return exportMsg{path: path}
	}
}

func opt(v *float64) string {
	if v == nil {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSuffix(strconv.FormatFloat(*v, 'f', 3, 64), "0"), ".")
}
