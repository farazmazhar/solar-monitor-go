package ui

import (
	"testing"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func TestDaySlotValues(t *testing.T) {
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.Local)
	samples := []provider.Sample{
		{TS: day.Add(13*time.Hour + 20*time.Minute), SolarW: provider.Float(5600)},
		{TS: day.Add(13*time.Hour + 25*time.Minute), SolarW: provider.Float(5500)},
	}
	values := daySlotValues(samples, func(s provider.Sample) *float64 { return s.SolarW })
	if len(values) != daySlotCount {
		t.Fatalf("len = %d, want %d", len(values), daySlotCount)
	}
	slot := 13*12 + 20/5
	if values[slot] == nil || *values[slot] != 5600 {
		t.Errorf("slot %d = %v, want 5600", slot, values[slot])
	}
	if values[0] != nil {
		t.Errorf("slot 0 should be empty, got %v", values[0])
	}
}

func TestRollingValuesKeepsMeasuredPoints(t *testing.T) {
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.Local)
	samples := []provider.Sample{
		{TS: day, LoadW: provider.Float(400)},
		{TS: day.Add(5 * time.Minute), LoadW: provider.Float(420)},
	}
	values := rollingValues(samples, func(s provider.Sample) *float64 { return s.LoadW })
	if len(values) != 2 {
		t.Fatalf("len = %d, want 2", len(values))
	}
}

func TestRenderLineHitRectInsideFrame(t *testing.T) {
	lines, hit := RenderLine(LineOptions{
		Width: 60, Height: 12,
		Title: "TEST", Border: true,
		Series:    []LineSeries{{Values: []*float64{provider.Float(1), provider.Float(2)}}},
		ShowYAxis: true, ShowXAxis: true,
		XLabels: []string{"00:00", "12:00", "23:55"},
	})
	if len(lines) != 12 {
		t.Fatalf("lines = %d, want 12", len(lines))
	}
	if hit.X < 1 || hit.Y < 1 || hit.X+hit.W > 59 || hit.Y+hit.H > 11 {
		t.Errorf("hit rect %+v is outside the framed area", hit)
	}
}

func TestDistributeLabelsWidth(t *testing.T) {
	got := distributeLabels([]string{"00:00", "06:00", "12:00", "18:00", "23:55"}, 40)
	if len([]rune(got)) != 40 {
		t.Fatalf("width = %d, want 40", len([]rune(got)))
	}
	if got[:5] != "00:00" {
		t.Errorf("first label = %q", got[:5])
	}
}

func TestComputeDayStats(t *testing.T) {
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.Local)
	var samples []provider.Sample
	for i := 0; i < 12; i++ {
		samples = append(samples, provider.Sample{
			TS:       day.Add(time.Duration(i) * 5 * time.Minute),
			SolarW:   provider.Float(3000),
			LoadW:    provider.Float(500),
			ImportW:  provider.Float(0),
			ExportW:  provider.Float(2500),
			BatteryW: provider.Float(0),
		})
	}
	st := computeDayStats(samples)
	if diff := st.generated - 3.0; diff < -0.01 || diff > 0.01 { // 12 * 3 kW * 5/60 h
		t.Errorf("generated = %v, want 3.0", st.generated)
	}
	if diff := st.exported - 2.5; diff < -0.01 || diff > 0.01 {
		t.Errorf("exported = %v, want 2.5", st.exported)
	}
	if st.peak != 3000 {
		t.Errorf("peak = %v, want 3000", st.peak)
	}
}

func TestCursorSlotAndSampleIndex(t *testing.T) {
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.Local)
	m := &Model{day: day, cursor: day.Add(13*time.Hour + 20*time.Minute)}
	if got := m.cursorSlot(day); got != 13*12+4 {
		t.Errorf("cursorSlot = %d, want %d", got, 13*12+4)
	}
	samples := []provider.Sample{
		{TS: day.Add(13 * time.Hour), LoadW: provider.Float(1)},
		{TS: day.Add(13*time.Hour + 20*time.Minute), LoadW: provider.Float(2)},
	}
	m.daySamples = samples
	if got := m.cursorSampleIndex(samples); got != 1 {
		t.Errorf("cursorSampleIndex = %d, want 1", got)
	}
}
