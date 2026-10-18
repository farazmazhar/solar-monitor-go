package ui

import (
	"strings"
	"testing"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func TestBoxGlyphMapping(t *testing.T) {
	cases := []struct {
		bits uint16
		want rune
	}{
		{boxE | boxW, '─'},
		{boxN | boxS, '│'},
		{boxNE, '╱'},
		{boxSW, '╱'},
		{boxNW, '╲'},
		{boxSE, '╲'},
		{boxN | boxE, '└'},
		{boxS | boxW, '┐'},
		{boxN | boxS | boxE | boxW, '┼'},
		{boxDot, '•'},
	}
	for _, c := range cases {
		if got := boxGlyph(c.bits); got != c.want {
			t.Errorf("boxGlyph(%08b) = %q, want %q", c.bits, got, c.want)
		}
	}
}

func TestBoxChartRisingLine(t *testing.T) {
	vals := make([]*float64, 64)
	for i := range vals {
		vals[i] = provider.Float(float64(i))
	}
	lines, _ := RenderLine(LineOptions{
		Width: 24, Height: 8, Style: ChartBox,
		Series: []LineSeries{{Values: vals}},
	})
	joined := strings.Join(lines, "\n")
	if !strings.ContainsAny(joined, "╱╲│└┘┌┐") {
		t.Errorf("rising line should contain diagonals or verticals, got:\n%s", joined)
	}
}
