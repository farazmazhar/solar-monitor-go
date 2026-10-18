package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/farazmazhar/solar-moonitor-go/internal/provider"
	"github.com/farazmazhar/solar-moonitor-go/internal/store"
)

type fakeSource struct {
	snap      provider.Snapshot
	snapErr   error
	day       []provider.Sample
	dayErr    error
	month     []provider.DayEnergy
	monthErr  error
	year      []provider.MonthEnergy
	yearErr   error
	alarms    []provider.Alarm
	alarmsErr error
}

func (f *fakeSource) Name() string { return "fake" }
func (f *fakeSource) Close() error { return nil }
func (f *fakeSource) Snapshot(context.Context) (provider.Snapshot, error) {
	return f.snap, f.snapErr
}
func (f *fakeSource) Today(context.Context, time.Time) ([]provider.Sample, error) {
	return f.day, f.dayErr
}
func (f *fakeSource) Month(context.Context, int, time.Month) ([]provider.DayEnergy, error) {
	return f.month, f.monthErr
}
func (f *fakeSource) Year(context.Context, int) ([]provider.MonthEnergy, error) {
	return f.year, f.yearErr
}
func (f *fakeSource) Alarms(context.Context, time.Time, time.Time) ([]provider.Alarm, error) {
	return f.alarms, f.alarmsErr
}

func newService(t *testing.T, src provider.Source) *Service {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return New(src, st, 30)
}

func TestSnapshotFallsBackToCache(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 10, 18, 13, 20, 0, 0, time.Local)
	src := &fakeSource{snap: provider.Snapshot{
		FetchedAt: base, DataAt: base, Source: "fake", Station: "Home",
		SolarW: provider.Float(5000),
	}}
	svc := newService(t, src)

	first, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first.SolarW == nil || *first.SolarW != 5000 {
		t.Fatalf("solar = %v", first.SolarW)
	}

	src.snapErr = errors.New("network down")
	cached, err := svc.Snapshot(ctx)
	if err != nil {
		t.Fatalf("cached snapshot should be returned, got %v", err)
	}
	if cached.SolarW == nil || *cached.SolarW != 5000 {
		t.Errorf("cached solar = %v", cached.SolarW)
	}
	if cached.Source != "fake (cached)" {
		t.Errorf("source = %q", cached.Source)
	}
}

func TestSnapshotWithNoCacheReturnsError(t *testing.T) {
	src := &fakeSource{snapErr: errors.New("down")}
	svc := newService(t, src)
	if _, err := svc.Snapshot(context.Background()); err == nil {
		t.Fatal("want the provider error when nothing is cached")
	}
}

func TestTodayMergesLocalAndRemote(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.Local)
	t1 := day.Add(10 * time.Hour)
	t2 := day.Add(11 * time.Hour)
	t3 := day.Add(12 * time.Hour)

	src := &fakeSource{
		snap: provider.Snapshot{FetchedAt: t3, DataAt: t3, Source: "fake", SolarW: provider.Float(1)},
		day: []provider.Sample{
			{TS: t1, SolarW: provider.Float(100)},
			{TS: t2, SolarW: provider.Float(200)},
		},
	}
	svc := newService(t, src)
	if _, err := svc.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	// The stored sample at t2 is the device reading and must win; t3 is local only.
	if err := svc.store.SaveSnapshot(ctx, provider.Snapshot{
		FetchedAt: t3, DataAt: t3, Source: "fake", SolarW: provider.Float(300),
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SaveSnapshot(ctx, provider.Snapshot{
		FetchedAt: t2, DataAt: t2, Source: "fake", SolarW: provider.Float(250),
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := svc.Today(ctx, day)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("merged rows = %d, want 3", len(rows))
	}
	if rows[1].SolarW == nil || *rows[1].SolarW != 250 {
		t.Errorf("t2 solar = %v, want the local 250", rows[1].SolarW)
	}
}

func TestMonthAndYearFallBackToStore(t *testing.T) {
	ctx := context.Background()
	src := &fakeSource{
		snap:  provider.Snapshot{FetchedAt: time.Now(), DataAt: time.Now(), Source: "fake"},
		month: []provider.DayEnergy{{Date: time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local), GeneratedKWh: provider.Float(21)}},
		year:  []provider.MonthEnergy{{Month: time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local), GeneratedKWh: provider.Float(866)}},
	}
	svc := newService(t, src)
	if _, err := svc.Month(ctx, 2026, time.October); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Year(ctx, 2026); err != nil {
		t.Fatal(err)
	}

	src.monthErr = errors.New("down")
	src.yearErr = errors.New("down")
	months, err := svc.Month(ctx, 2026, time.October)
	if err != nil {
		t.Fatalf("month fallback: %v", err)
	}
	if len(months) != 1 || months[0].GeneratedKWh == nil || *months[0].GeneratedKWh != 21 {
		t.Fatalf("months = %+v", months)
	}
	years, err := svc.Year(ctx, 2026)
	if err != nil {
		t.Fatalf("year fallback: %v", err)
	}
	if len(years) != 1 {
		t.Fatalf("years = %+v", years)
	}
}

func TestSeedReturnsCachedData(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 10, 18, 13, 20, 0, 0, time.Local)
	src := &fakeSource{snap: provider.Snapshot{FetchedAt: base, DataAt: base, Source: "fake", SolarW: provider.Float(42)}}
	svc := newService(t, src)
	if _, err := svc.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	snap, samples, ok := svc.Seed(ctx)
	if !ok {
		t.Fatal("seed should find the cached snapshot")
	}
	if snap.SolarW == nil || *snap.SolarW != 42 {
		t.Errorf("seed solar = %v", snap.SolarW)
	}
	if len(samples) != 1 {
		t.Errorf("seed samples = %d, want 1", len(samples))
	}
}
