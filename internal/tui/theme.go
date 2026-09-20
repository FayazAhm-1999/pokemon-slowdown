package tui

import (
	"charm.land/lipgloss/v2"
)

// Theme is a complete colour scheme. Gameplay never depends on colour alone:
// statuses and mechanics always carry a textual indicator too.
type Theme struct {
	Name string

	Fg      lipgloss.Style
	Muted   lipgloss.Style
	Dim     lipgloss.Style
	Primary lipgloss.Style
	Accent  lipgloss.Style
	Danger  lipgloss.Style
	Success lipgloss.Style
	Warning lipgloss.Style
	Inverse lipgloss.Style

	Title    lipgloss.Style
	Box      lipgloss.Style
	BoxFocus lipgloss.Style
	Selected lipgloss.Style
	Disabled lipgloss.Style

	HPFull lipgloss.Style
	HPHalf lipgloss.Style
	HPLow  lipgloss.Style

	Statuses map[string]lipgloss.Style
	Types    map[string]lipgloss.Style

	Border lipgloss.Border
}

// ThemeNames lists the shipped themes.
var ThemeNames = []string{"zen", "dark", "light", "gameboy", "mono", "contrast"}

// LoadTheme returns a theme by name, falling back to zen.
func LoadTheme(name string) Theme {
	switch name {
	case "dark":
		return darkTheme()
	case "light":
		return lightTheme()
	case "gameboy":
		return gameboyTheme()
	case "mono":
		return monoTheme()
	case "contrast":
		return contrastTheme()
	default:
		return zenTheme()
	}
}

func style(fg string, bold bool) lipgloss.Style {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color(fg))
	if bold {
		s = s.Bold(true)
	}
	return s
}

func buildTheme(name string, pal palette) Theme {
	t := Theme{
		Name:    name,
		Border:  lipgloss.RoundedBorder(),
		Fg:      style(pal.fg, false),
		Muted:   style(pal.muted, false),
		Dim:     style(pal.muted, false).Faint(true),
		Primary: style(pal.primary, true),
		Accent:  style(pal.accent, true),
		Danger:  style(pal.danger, true),
		Success: style(pal.success, false),
		Warning: style(pal.warning, false),
		Inverse: lipgloss.NewStyle().Background(lipgloss.Color(pal.primary)).Foreground(lipgloss.Color(pal.bg)),
		Title:   style(pal.primary, true),
		Box: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(pal.muted)),
		BoxFocus: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(pal.primary)),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color(pal.selection)).
			Foreground(lipgloss.Color(pal.fg)),
		Disabled: style(pal.muted, false).Faint(true),
		HPFull:   style(pal.success, false),
		HPHalf:   style(pal.warning, false),
		HPLow:    style(pal.danger, false),
		Statuses: map[string]lipgloss.Style{},
		Types:    map[string]lipgloss.Style{},
	}
	for _, s := range []string{"brn", "par", "slp", "frz", "psn", "tox", "fnt"} {
		t.Statuses[s] = style(pal.danger, true)
	}
	t.Statuses["par"] = style(pal.warning, true)
	t.Statuses["slp"] = style(pal.muted, true)
	t.Statuses["frz"] = style(pal.accent, true)
	t.Statuses["fnt"] = style(pal.muted, true).Faint(true)

	for typ, col := range typeColors {
		t.Types[typ] = lipgloss.NewStyle().
			Background(lipgloss.Color(col)).
			Foreground(lipgloss.Color("#1b1b1b")).
			Padding(0, 1)
	}
	return t
}

type palette struct {
	bg        string
	fg        string
	muted     string
	primary   string
	accent    string
	danger    string
	success   string
	warning   string
	selection string
}

func zenTheme() Theme {
	return buildTheme("zen", palette{
		bg: "#1c1b22", fg: "#e8e3d9", muted: "#8a8578",
		primary: "#c9a0dc", accent: "#f0a868", danger: "#e06c75",
		success: "#98c379", warning: "#e5c07b", selection: "#3a3550",
	})
}

func darkTheme() Theme {
	return buildTheme("dark", palette{
		bg: "#101216", fg: "#c8ccd4", muted: "#5c6370",
		primary: "#7aa2f7", accent: "#bb9af7", danger: "#f7768e",
		success: "#9ece6a", warning: "#e0af68", selection: "#2a2f3a",
	})
}

func lightTheme() Theme {
	t := buildTheme("light", palette{
		bg: "#faf8f3", fg: "#2b2b2b", muted: "#8a8578",
		primary: "#6b4ea0", accent: "#b5651d", danger: "#b3261e",
		success: "#2e7d32", warning: "#a16207", selection: "#e6ddc9",
	})
	t.Border = lipgloss.NormalBorder()
	return t
}

func gameboyTheme() Theme {
	t := buildTheme("gameboy", palette{
		bg: "#0f380f", fg: "#9bbc0f", muted: "#306230",
		primary: "#8bac0f", accent: "#9bbc0f", danger: "#306230",
		success: "#8bac0f", warning: "#9bbc0f", selection: "#306230",
	})
	t.Border = lipgloss.NormalBorder()
	return t
}

func monoTheme() Theme {
	t := buildTheme("mono", palette{
		bg: "#000000", fg: "#d0d0d0", muted: "#707070",
		primary: "#ffffff", accent: "#ffffff", danger: "#ffffff",
		success: "#d0d0d0", warning: "#d0d0d0", selection: "#303030",
	})
	t.Border = lipgloss.NormalBorder()
	return t
}

func contrastTheme() Theme {
	t := buildTheme("contrast", palette{
		bg: "#000000", fg: "#ffffff", muted: "#c0c0c0",
		primary: "#ffff00", accent: "#00ffff", danger: "#ff0000",
		success: "#00ff00", warning: "#ff8000", selection: "#0000ff",
	})
	t.Border = lipgloss.ThickBorder()
	return t
}

// typeColors maps Pokémon types to their canonical colours.
var typeColors = map[string]string{
	"normal":   "#a8a878",
	"fire":     "#f08030",
	"water":    "#6890f0",
	"electric": "#f8d030",
	"grass":    "#78c850",
	"ice":      "#98d8d8",
	"fighting": "#c03028",
	"poison":   "#a040a0",
	"ground":   "#e0c068",
	"flying":   "#a890f0",
	"psychic":  "#f85888",
	"bug":      "#a8b820",
	"rock":     "#b8a038",
	"ghost":    "#705898",
	"dragon":   "#7038f8",
	"dark":     "#705848",
	"steel":    "#b8b8d0",
	"fairy":    "#ee99ac",
}
