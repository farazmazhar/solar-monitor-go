package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func TestMixSeriesSplitsLoad(t *testing.T) {
	day := time.Date(2026, 10, 18, 0, 0, 0, 0, time.Local)
	m := &Model{}
	samples := []provider.Sample{
		{TS: day.Add(12 * time.Hour), SolarW: provider.Float(5000), LoadW: provider.Float(1000), BatteryW: provider.Float(0), GridW: provider.Float(4000)},
		{TS: day.Add(20 * time.Hour), SolarW: provider.Float(0), LoadW: provider.Float(1000), BatteryW: provider.Float(-600), GridW: provider.Float(-400)},
	}
	layers := m.mixSeries(samples, false, day)
	if len(layers) != 3 {
		t.Fatalf("layers = %d, want 3", len(layers))
	}
	// layers are grid, battery, solar
	if layers[2].Values[0] == nil || *layers[2].Values[0] != 1000 {
		t.Errorf("midday solar-to-load = %v, want 1000", layers[2].Values[0])
	}
	if layers[1].Values[1] == nil || *layers[1].Values[1] != 600 {
		t.Errorf("evening battery-to-load = %v, want 600", layers[1].Values[1])
	}
	if layers[0].Values[1] == nil || *layers[0].Values[1] != 400 {
		t.Errorf("evening grid-to-load = %v, want 400", layers[0].Values[1])
	}
}

func TestRenderStackFrame(t *testing.T) {
	vals := []*float64{provider.Float(1), provider.Float(2), provider.Float(3)}
	lines, hit := RenderStack(StackOptions{
		Width: 40, Height: 10, Title: "MIX", Border: true,
		Series:  []StackSeries{{Name: "solar", Values: vals}},
		XLabels: []string{"00:00", "12:00", "23:55"},
	})
	if len(lines) != 10 {
		t.Fatalf("lines = %d, want 10", len(lines))
	}
	if hit.W <= 0 || hit.H <= 0 {
		t.Errorf("hit = %+v", hit)
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "MIX") {
		t.Errorf("title missing:\n%s", joined)
	}
}

func TestByteShade(t *testing.T) {
	cases := []struct {
		v    float64
		want rune
	}{
		{0, ' '},
		{1, '░'},
		{30, '▒'},
		{60, '▓'},
		{90, '█'},
	}
	for _, c := range cases {
		if got := byteShade(c.v, 100); got != c.want {
			t.Errorf("byteShade(%v) = %q, want %q", c.v, got, c.want)
		}
	}
}
