package demo

import (
	"context"
	"testing"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func fixed(t time.Time) *Provider {
	return New(Options{Clock: func() time.Time { return t }})
}

func TestSnapshotIsStableForAFixedClock(t *testing.T) {
	at := time.Date(2026, 10, 18, 13, 20, 0, 0, time.Local)
	p := fixed(at)
	snap, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.SolarW == nil || *snap.SolarW <= 0 {
		t.Fatalf("solar = %v, want a positive value", snap.SolarW)
	}
	if snap.SOC == nil || *snap.SOC <= 0 || *snap.SOC > 100 {
		t.Fatalf("soc = %v", snap.SOC)
	}
	if snap.LoadW == nil || *snap.LoadW <= 0 {
		t.Fatalf("load = %v", snap.LoadW)
	}
	if !snap.Flow.Balanced {
		t.Errorf("flow should balance: %+v", snap.Flow)
	}
	if snap.State != provider.StateOnline {
		t.Errorf("state = %q", snap.State)
	}
	if len(snap.PVStrings) != 2 {
		t.Errorf("strings = %d, want 2", len(snap.PVStrings))
	}
	if len(snap.Raw) == 0 {
		t.Error("demo should expose a raw payload for the diagnostics browser")
	}

	// A second call at the same instant must return the same numbers.
	again, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *again.SolarW != *snap.SolarW || *again.SOC != *snap.SOC {
		t.Errorf("snapshot is not stable: %v vs %v", again.SolarW, snap.SolarW)
	}
}

func TestTodayMonthYearAlarms(t *testing.T) {
	at := time.Date(2026, 10, 18, 13, 20, 0, 0, time.Local)
	p := fixed(at)
	ctx := context.Background()

	day, err := p.Today(ctx, at)
	if err != nil {
		t.Fatal(err)
	}
	if len(day) == 0 {
		t.Fatal("today returned no samples")
	}
	if got := len(day); got != 161 { // midnight to 13:20 inclusive at five minutes
		t.Errorf("today samples = %d, want 161", got)
	}

	month, err := p.Month(ctx, 2026, time.October)
	if err != nil {
		t.Fatal(err)
	}
	if len(month) != 18 {
		t.Errorf("month rows = %d, want 18", len(month))
	}

	year, err := p.Year(ctx, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(year) != 10 {
		t.Errorf("year rows = %d, want 10", len(year))
	}

	alarms, err := p.Alarms(ctx, at.AddDate(0, 0, -30), at)
	if err != nil {
		t.Fatal(err)
	}
	if len(alarms) != 3 {
		t.Errorf("alarms = %d, want 3", len(alarms))
	}
	for _, a := range alarms {
		if a.Active {
			t.Errorf("demo alarms should be restored, got %+v", a)
		}
	}
}

func TestSolarIsZeroAtNight(t *testing.T) {
	night := time.Date(2026, 10, 18, 2, 0, 0, 0, time.Local)
	snap, err := fixed(night).Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snap.SolarW == nil || *snap.SolarW != 0 {
		t.Errorf("solar at night = %v, want 0", snap.SolarW)
	}
}
