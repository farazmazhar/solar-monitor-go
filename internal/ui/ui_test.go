package ui

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/farazmazhar/solar-moonitor-go/internal/config"
	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
	"github.com/farazmazhar/solar-moonitor-go/internal/provider/demo"
)

var update = flag.Bool("update", false, "update golden files")

var testClock = time.Date(2026, 10, 18, 13, 20, 0, 0, time.Local)

func testProvider() provider.Source {
	return demo.New(demo.Options{Clock: func() time.Time { return testClock }})
}

func testModel(t *testing.T, w, h int, tab Tab, help bool) *Model {
	t.Helper()
	cfg := config.Defaults()
	cfg.Demo.Enabled = true
	cfg.Display.Motion = false
	paths := config.Paths{
		ConfigPath: "/home/user/.config/solar-moonitor/config.toml",
		StateDir:   t.TempDir(),
		DBPath:     "/home/user/.local/state/solar-moonitor/monitor.db",
	}
	src := testProvider()
	m := New(Options{Config: cfg, Paths: paths, Source: src, Demo: true, Version: "test"})
	m.clock = func() time.Time { return testClock }
	m.width, m.height = w, h
	m.tab = tab
	m.help = help

	ctx := context.Background()
	snap, err := src.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m.snap = snap
	m.haveSnap = true
	m.appendSample(snap)
	if day, err := src.Today(ctx, testClock); err == nil {
		m.daySamples = day
		m.day = testClock
	}
	if month, err := src.Month(ctx, 2026, time.October); err == nil {
		m.month = month
		m.monthAt = time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	}
	if year, err := src.Year(ctx, 2026); err == nil {
		m.year = year
		m.yearAt = time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	}
	if alarms, err := src.Alarms(ctx, testClock.AddDate(0, 0, -30), testClock); err == nil {
		m.alarms = alarms
		m.alarmsAt = testClock
	}
	m.nextPoll = testClock.Add(2 * time.Second)
	return m
}

func renderFrame(t *testing.T, m *Model) string {
	t.Helper()
	plain := ansi.Strip(m.View().Content)
	lines := strings.Split(plain, "\n")
	if len(lines) != m.height {
		t.Fatalf("frame height = %d, want %d", len(lines), m.height)
	}
	for i, line := range lines {
		if got := ansi.StringWidth(line); got > m.width {
			t.Errorf("line %d width = %d, want <= %d: %q", i, got, m.width, line)
		}
	}
	return plain
}

func TestGoldenFrames(t *testing.T) {
	cases := []struct {
		name string
		w, h int
		tab  Tab
		help bool
	}{
		{"live_80x24", 80, 24, TabLive, false},
		{"today_80x24", 80, 24, TabToday, false},
		{"history_80x24", 80, 24, TabHistory, false},
		{"battery_80x24", 80, 24, TabBattery, false},
		{"pv_80x24", 80, 24, TabPV, false},
		{"alarms_80x24", 80, 24, TabAlarms, false},
		{"diagnostics_80x24", 80, 24, TabDiagnostics, false},
		{"live_60x20", 60, 20, TabLive, false},
		{"live_100x30", 100, 30, TabLive, false},
		{"help_80x24", 80, 24, TabLive, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := testModel(t, tc.w, tc.h, tc.tab, tc.help)
			got := renderFrame(t, m)
			path := filepath.Join("testdata", tc.name+".golden")
			if *update {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (run go test ./internal/ui -update)", err)
			}
			if got != string(want) {
				t.Errorf("frame does not match %s\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

func TestSetupScreen(t *testing.T) {
	cfg := config.Defaults()
	m := New(Options{
		Config:   cfg,
		Paths:    config.Paths{ConfigPath: "/home/user/.config/solar-moonitor/config.toml", StateDir: t.TempDir()},
		SetupErr: errSetup,
		Version:  "test",
	})
	m.width, m.height = 80, 24
	frame := renderFrame(t, m)
	if !strings.Contains(frame, "Nothing to monitor yet") {
		t.Errorf("setup screen missing its headline:\n%s", frame)
	}
}

var errSetup = &setupError{}

type setupError struct{}

func (*setupError) Error() string { return "solis.api_key is empty" }

func TestTransparentThemeHasNoBackground(t *testing.T) {
	th := NewTheme("transparent")
	if th.Bg != nil {
		t.Error("transparent theme must not paint a background")
	}
	if NewTheme("nonsense").Name != "solar-dark" {
		t.Error("unknown themes should fall back to solar-dark")
	}
}
