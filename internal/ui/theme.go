package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// Theme holds every colour and style the UI uses. View code must take colours
// from here rather than declaring literals.
type Theme struct {
	Name string

	Bg, Fg, Dim, Faint      color.Color
	Solar, Green            color.Color
	Grid, Export, Import    color.Color
	Battery, Discharge      color.Color
	Accent, Warning, Danger color.Color

	AppName     lipgloss.Style
	HeaderMeta  lipgloss.Style
	TabActive   lipgloss.Style
	TabInactive lipgloss.Style
	Label       lipgloss.Style
	Value       lipgloss.Style
	Muted       lipgloss.Style
	Rule        lipgloss.Style
	Good        lipgloss.Style
	Warn        lipgloss.Style
	Bad         lipgloss.Style
	Selected    lipgloss.Style
	HelpKey     lipgloss.Style
	HelpDesc    lipgloss.Style
}

// NewTheme resolves a theme name. Unknown names fall back to solar-dark.
func NewTheme(name string) Theme {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "transparent", "none":
		t := solarPalette()
		t.Name = "transparent"
		t.Bg = nil
		return t
	default:
		t := solarPalette()
		t.Name = "solar-dark"
		return t
	}
}

func solarPalette() Theme {
	bg := lipgloss.Color("#0B0F14")
	fg := lipgloss.Color("#E6EDF3")
	dim := lipgloss.Color("#8B98A5")
	faint := lipgloss.Color("#22303C")
	solar := lipgloss.Color("#F5A524")
	green := lipgloss.Color("#22C55E")
	grid := lipgloss.Color("#4C8DF6")
	export := lipgloss.Color("#22D3EE")
	imp := lipgloss.Color("#F97316")
	battery := lipgloss.Color("#A78BFA")
	discharge := lipgloss.Color("#F472B6")
	accent := lipgloss.Color("#F5A524")
	warning := lipgloss.Color("#FBBF24")
	danger := lipgloss.Color("#EF4444")

	t := Theme{
		Name:      "solar-dark",
		Bg:        bg,
		Fg:        fg,
		Dim:       dim,
		Faint:     faint,
		Solar:     solar,
		Green:     green,
		Grid:      grid,
		Export:    export,
		Import:    imp,
		Battery:   battery,
		Discharge: discharge,
		Accent:    accent,
		Warning:   warning,
		Danger:    danger,

		AppName:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		HeaderMeta:  lipgloss.NewStyle().Foreground(dim),
		TabActive:   lipgloss.NewStyle().Bold(true).Foreground(bg).Background(accent),
		TabInactive: lipgloss.NewStyle().Foreground(dim),
		Label:       lipgloss.NewStyle().Foreground(dim),
		Value:       lipgloss.NewStyle().Bold(true).Foreground(fg),
		Muted:       lipgloss.NewStyle().Foreground(dim),
		Rule:        lipgloss.NewStyle().Foreground(faint),
		Good:        lipgloss.NewStyle().Foreground(green),
		Warn:        lipgloss.NewStyle().Foreground(warning),
		Bad:         lipgloss.NewStyle().Foreground(danger),
		Selected:    lipgloss.NewStyle().Bold(true).Foreground(bg).Background(accent),
		HelpKey:     lipgloss.NewStyle().Bold(true).Foreground(accent),
		HelpDesc:    lipgloss.NewStyle().Foreground(dim),
	}
	return t
}

// Style returns a foreground style for a token.
func (t Theme) Style(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

// Bold returns a bold foreground style for a token.
func (t Theme) Bold(c color.Color) lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(c)
}

// Pill renders a compact state badge.
func (t Theme) Pill(text string, fg color.Color) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(fg).
		Background(t.pillBg(fg)).
		Padding(0, 1).
		Render(text)
}

func (t Theme) pillBg(fg color.Color) color.Color {
	// The palette has no per-colour background tokens; a near-black chip keeps
	// the foreground readable while staying inside the theme.
	if t.Bg == nil {
		return lipgloss.Color("#1B2430")
	}
	return lipgloss.Color("#141B24")
}
