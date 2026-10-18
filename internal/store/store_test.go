package store

import (
	"context"
	"testing"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func TestStoreRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	base := time.Date(2026, 10, 18, 13, 20, 0, 0, time.Local)
	snap := provider.Snapshot{
		FetchedAt: base,
		DataAt:    base.Truncate(5 * time.Minute),
		Source:    "test",
		Station:   "Home",
		SN:        "SN1",
		SolarW:    provider.Float(5500),
		LoadW:     provider.Float(650),
		GridW:     provider.Float(3200),
		BatteryW:  provider.Float(1800),
		SOC:       provider.Float(62),
		TodayKWh:  provider.Float(25.9),
	}
	if err := s.SaveSnapshot(ctx, snap); err != nil {
		t.Fatal(err)
	}
	// Same sample again must replace, not duplicate.
	if err := s.SaveSnapshot(ctx, snap); err != nil {
		t.Fatal(err)
	}

	got, ok, err := s.LatestSnapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected a stored snapshot")
	}
	if got.SolarW == nil || *got.SolarW != 5500 {
		t.Errorf("solar = %v", got.SolarW)
	}
	if got.Station != "Home" {
		t.Errorf("station = %q", got.Station)
	}

	samples, err := s.Samples(ctx, base.Add(-time.Hour), base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 {
		t.Fatalf("samples = %d, want 1", len(samples))
	}
	if samples[0].GridW == nil || *samples[0].GridW != 3200 {
		t.Errorf("grid = %v", samples[0].GridW)
	}

	stats, err := s.Stats(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Samples != 1 || stats.Oldest.IsZero() {
		t.Errorf("stats = %+v", stats)
	}
}

func TestStoreAggregates(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	days := []provider.DayEnergy{{
		Date:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local),
		GeneratedKWh: provider.Float(21.4),
		ConsumedKWh:  provider.Float(13),
		Income:       provider.Float(577.8),
		Currency:     "PKR",
	}}
	if err := s.SaveDayEnergy(ctx, days); err != nil {
		t.Fatal(err)
	}
	// Upsert overwrites.
	days[0].GeneratedKWh = provider.Float(22.5)
	if err := s.SaveDayEnergy(ctx, days); err != nil {
		t.Fatal(err)
	}
	rows, err := s.DayEnergy(ctx, 2026, time.October)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].GeneratedKWh == nil || *rows[0].GeneratedKWh != 22.5 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Currency != "PKR" {
		t.Errorf("currency = %q", rows[0].Currency)
	}

	months := []provider.MonthEnergy{{
		Month:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local),
		GeneratedKWh: provider.Float(866000),
	}}
	if err := s.SaveMonthEnergy(ctx, months); err != nil {
		t.Fatal(err)
	}
	mrows, err := s.MonthEnergy(ctx, 2026)
	if err != nil {
		t.Fatal(err)
	}
	if len(mrows) != 1 || mrows[0].GeneratedKWh == nil || *mrows[0].GeneratedKWh != 866000 {
		t.Fatalf("months = %+v", mrows)
	}
	if len(mustMonths(t, s, 2027)) != 0 {
		t.Error("unexpected rows for another year")
	}
}

func mustMonths(t *testing.T, s *Store, year int) []provider.MonthEnergy {
	t.Helper()
	rows, err := s.MonthEnergy(context.Background(), year)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestStoreAlarms(t *testing.T) {
	ctx := context.Background()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	begin := time.Date(2026, 10, 16, 9, 20, 0, 0, time.Local)
	alarms := []provider.Alarm{{
		ID: "a1", Level: 2, Code: "1030", Message: "Grid voltage high",
		Advice: "Wait", State: "restored", Begin: begin, End: begin.Add(18 * time.Minute),
	}}
	if err := s.SaveAlarms(ctx, alarms); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Alarms(ctx, begin.AddDate(0, 0, -1), begin.AddDate(0, 0, 1))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "1030" || rows[0].Active {
		t.Fatalf("alarms = %+v", rows)
	}
}

func TestStoreFilePersists(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()
	path := dir + "/monitor.db"

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().Truncate(time.Minute)
	if err := s.SaveSnapshot(ctx, provider.Snapshot{FetchedAt: base, DataAt: base, Source: "test", SolarW: provider.Float(1)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	again, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	if _, ok, err := again.LatestSnapshot(ctx); err != nil || !ok {
		t.Fatalf("snapshot missing after reopen: ok=%v err=%v", ok, err)
	}
}
