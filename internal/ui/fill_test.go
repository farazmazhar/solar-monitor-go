package ui

import (
	"strings"
	"testing"

	"github.com/farazmazhar/solar-monitor-go/internal/provider"
)

func TestAreaFillRenders(t *testing.T) {
	vals := make([]*float64, 100)
	for i := range vals {
		vals[i] = provider.Float(float64(i % 50))
	}
	lines, _ := RenderLine(LineOptions{Width: 50, Height: 10, Fill: true, FillSeries: 0, FillGlyph: '░', Series: []LineSeries{{Values: vals}}})
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "░") {
		t.Errorf("no fill glyph:\n%s", joined)
	}
}
