package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
	"github.com/farazmazhar/solar-monitor-go/internal/store"
)

type fakeSource struct {
	snap       provider.Snapshot
	snapErr    error
	day        []provider.Sample
	dayErr     error
	month      []provider.DayEnergy
	monthErr   error
	monthCalls int
	year       []provider.MonthEnergy
	yearErr    error
	alarms     []provider.Alarm
	alarmsErr  error
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
	f.monthCalls++
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

func TestExpectedDays(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.Local)
	if got := expectedDays(2026, time.September, now); got != 30 {
		t.Errorf("September = %d, want 30", got)
	}
	if got := expectedDays(2026, time.October, now); got != 18 {
		t.Errorf("current month = %d, want 18", got)
	}
	if got := expectedDays(2026, time.December, now); got != 0 {
		t.Errorf("future month = %d, want 0", got)
	}
	if got := expectedDays(2024, time.February, now); got != 29 {
		t.Errorf("leap February = %d, want 29", got)
	}
}

func TestYearDaysUsesCache(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Seed a complete past year.
	var days []provider.DayEnergy
	for mo := time.January; mo <= time.December; mo++ {
		n := time.Date(2025, mo+1, 0, 0, 0, 0, 0, time.Local).Day()
		for d := 1; d <= n; d++ {
			days = append(days, provider.DayEnergy{
				Date:         time.Date(2025, mo, d, 0, 0, 0, 0, time.Local),
				GeneratedKWh: provider.Float(10),
			})
		}
	}
	if err := st.SaveDayEnergy(ctx, days); err != nil {
		t.Fatal(err)
	}

	src := &fakeSource{}
	svc := New(src, st, 30)
	svc.pace = 0
	rows, err := svc.YearDays(ctx, 2025)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 365 {
		t.Fatalf("rows = %d, want 365", len(rows))
	}
	if src.monthCalls != 0 {
		t.Errorf("month calls = %d, want 0 when the cache is complete", src.monthCalls)
	}
}

func TestYearDaysFetchesMissingMonths(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	src := &fakeSource{month: []provider.DayEnergy{{Date: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local), GeneratedKWh: provider.Float(5)}}}
	svc := New(src, st, 30)
	svc.pace = 0
	if _, err := svc.YearDays(ctx, 2020); err != nil {
		t.Fatal(err)
	}
	if src.monthCalls != 12 {
		t.Errorf("month calls = %d, want 12 for an empty cache", src.monthCalls)
	}
}
