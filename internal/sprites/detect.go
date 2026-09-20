package sprites

import (
	"os"
	"strings"
)

// Protocol identifies a terminal graphics backend.
type Protocol string

// Supported protocols.
const (
	ProtocolNone   Protocol = "none"
	ProtocolBlocks Protocol = "blocks"
	ProtocolKitty  Protocol = "kitty"
	ProtocolITerm2 Protocol = "iterm2"
	ProtocolSixel  Protocol = "sixel"
)

// Capabilities describes what a backend can do in this terminal.
type Capabilities struct {
	// Protocol is the backend actually in use.
	Protocol Protocol
	// Detected is the pixel protocol the terminal appears to support, which
	// may differ from the active backend (for example when a user forces the
	// block fallback on a Kitty-capable terminal).
	Detected      Protocol
	PixelAccurate bool
	Animated      bool
	// CellWidth and CellHeight are the terminal cell size in pixels, or 0 when
	// unknown.
	CellWidth  int
	CellHeight int
	// Tmux is true when running inside a terminal multiplexer.
	Tmux bool
	// Term and TermProgram record what was detected, for diagnostics.
	Term        string
	TermProgram string
}

// Detect inspects the environment and returns the best backend it is confident
// about. Inside tmux it deliberately reports the block fallback, because
// graphics passthrough is frequently unreliable there.
func Detect() Capabilities {
	caps := Capabilities{
		Term:        os.Getenv("TERM"),
		TermProgram: os.Getenv("TERM_PROGRAM"),
		Tmux:        os.Getenv("TMUX") != "",
	}

	switch {
	case isKittyFamily():
		caps.Protocol = ProtocolKitty
		caps.PixelAccurate = true
	case isITerm2():
		caps.Protocol = ProtocolITerm2
		caps.PixelAccurate = true
	case isSixel():
		caps.Protocol = ProtocolSixel
		caps.PixelAccurate = true
	default:
		caps.Protocol = ProtocolBlocks
	}

	if caps.Tmux {
		caps.Protocol = ProtocolBlocks
		caps.PixelAccurate = false
	}
	caps.Detected = caps.Protocol
	return caps
}

func isKittyFamily() bool {
	if os.Getenv("KITTY_WINDOW_ID") != "" {
		return true
	}
	if os.Getenv("WEZTERM_PANE") != "" {
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	if strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") {
		return true
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "ghostty", "kitty", "WezTerm":
		return true
	}
	return false
}

func isITerm2() bool {
	return os.Getenv("TERM_PROGRAM") == "iTerm.app"
}

func isSixel() bool {
	term := strings.ToLower(os.Getenv("TERM"))
	for _, marker := range []string{"sixel", "mlterm", "foot", "contour", "yaft"} {
		if strings.Contains(term, marker) {
			return true
		}
	}
	return os.Getenv("COLORTERM") == "sixel"
}
