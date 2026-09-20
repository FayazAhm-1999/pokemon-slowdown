package tui

// LayoutMode is the semantic layout the current terminal width calls for. The
// UI never simply squeezes the widest layout: each mode is designed for its
// size, and the sidecar mode is specifically built for sitting beside an
// editor.
type LayoutMode int

// Layout modes, narrowest first.
const (
	LayoutCompact LayoutMode = iota
	LayoutSidecar
	LayoutStandard
	LayoutCinematic
)

// LayoutFor picks the layout for a terminal width. Thresholds are deliberately
// a little different from the original sketch after testing in a real pane.
func LayoutFor(cols int) LayoutMode {
	switch {
	case cols >= 100:
		return LayoutCinematic
	case cols >= 70:
		return LayoutStandard
	case cols >= 46:
		return LayoutSidecar
	default:
		return LayoutCompact
	}
}

// String returns a human name for the mode.
func (m LayoutMode) String() string {
	switch m {
	case LayoutCinematic:
		return "cinematic"
	case LayoutStandard:
		return "standard"
	case LayoutSidecar:
		return "sidecar"
	default:
		return "compact"
	}
}

// Sprites reports whether sprites should be drawn at all in this mode.
func (m LayoutMode) Sprites() bool { return m != LayoutCompact }

// SpriteCells returns the sprite box size in terminal cells for this mode.
func (m LayoutMode) SpriteCells() (cols, rows int) {
	switch m {
	case LayoutCinematic:
		return 18, 9
	case LayoutStandard:
		return 12, 6
	case LayoutSidecar:
		return 9, 5
	default:
		return 0, 0
	}
}

// ShowFieldConditions reports whether the field bar fits.
func (m LayoutMode) ShowFieldConditions() bool { return m != LayoutCompact }

// SpriteLayout reports whether the wide sprite-beside-detail battlefield is
// used, as opposed to a single line per Pokémon.
func (m LayoutMode) SpriteLayout() bool { return m == LayoutCinematic || m == LayoutStandard }

// MinWidth and MinHeight are the smallest usable terminal.
const (
	MinWidth  = 32
	MinHeight = 12
)

// TooSmall reports whether the terminal cannot host the UI at all.
func TooSmall(cols, rows int) bool {
	return cols < MinWidth || rows < MinHeight
}
