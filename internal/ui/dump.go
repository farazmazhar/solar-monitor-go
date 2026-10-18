package ui

import (
	"context"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/config"
	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

// DumpFrame renders one frame of the UI with live data fetched synchronously.
// It backs the --dump debugging flag and does not start Bubble Tea.
func DumpFrame(src provider.Source, cfg config.Config, paths config.Paths, version string, demo bool, tab Tab, history HistoryMode, w, h int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m := New(Options{Config: cfg, Paths: paths, Source: src, Demo: demo, Version: version, InitialTab: tab, InitialHistory: history})
	m.width, m.height = w, h
	m.th = NewTheme(cfg.Display.Theme)

	if snap, err := src.Snapshot(ctx); err == nil {
		m.snap = snap
		m.haveSnap = true
		m.appendSample(snap)
	}
	now := time.Now()
	if samples, err := src.Today(ctx, now); err == nil {
		m.daySamples = samples
		m.historySamples = samples
		m.day = now
		m.historyDay = now
	}
	if rows, err := src.Month(ctx, now.Year(), now.Month()); err == nil {
		m.month = rows
		m.monthAt = firstOfMonth(now)
	}
	if rows, err := src.Year(ctx, now.Year()); err == nil {
		m.year = rows
		m.yearAt = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	}
	if alarms, err := src.Alarms(ctx, now.AddDate(0, 0, -30), now); err == nil {
		m.alarms = alarms
		m.alarmsAt = now
	}
	m.nextPoll = time.Now().Add(m.pollDelay())
	return m.View().Content, nil
}
