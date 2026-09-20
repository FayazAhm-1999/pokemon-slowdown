// Package sprites owns terminal sprite rendering. It provides a small
// Renderer abstraction with four interchangeable backends, so the UI never
// depends on a third-party image library and a broken backend can be replaced
// without touching the application.
//
// Correctness never depends on advanced pixel protocols: the half-block
// backend is pure text and works in every terminal, and it is the default.
package sprites

import (
	"fmt"
	"image"
	"image/color"
)

// Renderer draws an image into a reserved terminal rectangle of cols x rows
// cells. Render returns the string to place at that position in the view.
type Renderer interface {
	// Capabilities describes the backend.
	Capabilities() Capabilities
	// Render encodes the image for a rectangle of cols x rows cells.
	Render(img image.Image, cols, rows int) (string, error)
	// Close releases terminal-side resources (for example, deleting images
	// placed by the Kitty protocol).
	Close() error
}

// Mode is a user-selectable backend preference.
type Mode string

// Renderer modes.
const (
	ModeAuto   Mode = "auto"
	ModeKitty  Mode = "kitty"
	ModeITerm2 Mode = "iterm2"
	ModeSixel  Mode = "sixel"
	ModeBlocks Mode = "blocks"
	ModeNone   Mode = "none"
)

// ParseMode converts a config string to a Mode, defaulting to auto.
func ParseMode(s string) Mode {
	switch Mode(s) {
	case ModeKitty, ModeITerm2, ModeSixel, ModeBlocks, ModeNone, ModeAuto:
		return Mode(s)
	default:
		return ModeAuto
	}
}

// NewRenderer builds the renderer for a mode. ModeAuto uses terminal
// detection. bg is the terminal background, used to composite transparency in
// the block backend.
func NewRenderer(mode Mode, bg color.Color) (Renderer, error) {
	detected := Detect()
	switch mode {
	case ModeNone:
		detected.Protocol = ProtocolNone
		detected.PixelAccurate = false
		return &noneRenderer{caps: detected}, nil
	case ModeBlocks:
		detected.Protocol = ProtocolBlocks
		detected.PixelAccurate = false
		return &halfBlockRenderer{caps: detected, bg: bg}, nil
	case ModeKitty:
		return newKittyRenderer(detected), nil
	case ModeITerm2:
		return newITerm2Renderer(detected), nil
	case ModeSixel:
		return newSixelRenderer(detected), nil
	}
	// Auto: half-block is the reliable default. Pixel backends are opt-in
	// because terminal graphics inside a full-screen TUI can be fragile.
	detected.Protocol = ProtocolBlocks
	detected.PixelAccurate = false
	return &halfBlockRenderer{caps: detected, bg: bg}, nil
}

// DetectedMode reports which pixel backend the environment appears to support,
// for display by the doctor command.
func DetectedMode() Protocol { return Detect().Protocol }

// noneRenderer renders nothing, for sprites.mode = "none".
type noneRenderer struct{ caps Capabilities }

func (r *noneRenderer) Capabilities() Capabilities { return r.caps }

func (r *noneRenderer) Render(image.Image, int, int) (string, error) { return "", nil }

func (r *noneRenderer) Close() error { return nil }

// fit scales img to fit inside w x h pixels using nearest-neighbour sampling,
// which keeps pixel art crisp, and returns the scaled image.
func fit(img image.Image, w, h int) *image.RGBA {
	b := img.Bounds()
	sw, sh := b.Dx(), b.Dy()
	if sw <= 0 || sh <= 0 || w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 0, 0))
	}

	nw, nh := w, h
	if sw*h > sh*w {
		// Wider than the box: constrain by width.
		nh = sh * w / sw
	} else {
		nw = sw * h / sh
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + y*sh/nh
		for x := 0; x < nw; x++ {
			sx := b.Min.X + x*sw/nw
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	return dst
}

// opaque reports whether a colour is meaningfully visible. A nil colour means
// "no colour set", which is not opaque.
func opaque(c color.Color) bool {
	if c == nil {
		return false
	}
	_, _, _, a := c.RGBA()
	return a >= 0x2000
}

// rgb renders a colour as the three components of a truecolour sequence.
func rgb(c color.Color) (r, g, b uint8) {
	rr, gg, bb, _ := c.RGBA()
	return uint8(rr >> 8), uint8(gg >> 8), uint8(bb >> 8)
}

// rgbString formats a colour for a 24-bit ANSI sequence.
func rgbString(c color.Color) string {
	r, g, b := rgb(c)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}
