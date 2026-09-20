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
	// Auto prefers a pixel backend, but only one we can actually drive: the
	// Kitty protocol is drawn out of band around Bubble Tea's renderer, which
	// otherwise strips graphics escapes. Everything else falls back to
	// half-blocks, which are pure text and always work.
	if detected.Detected == ProtocolKitty && !detected.Tmux {
		return newKittyRenderer(detected), nil
	}
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

// fit scales img to fit inside w x h pixels and returns the scaled image.
//
// Downscaling uses area averaging rather than point sampling: a 96x96 sprite
// rendered into an 18x18 box loses most of its pixels, and picking one pixel
// per destination cell aliases the artwork into noise. Averaging keeps the
// silhouette and colour of the original. Upscaling still uses nearest
// neighbour, which keeps pixel art crisp.
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
	if nw <= sw && nh <= sh {
		for y := 0; y < nh; y++ {
			y0 := b.Min.Y + y*sh/nh
			y1 := b.Min.Y + (y+1)*sh/nh
			if y1 <= y0 {
				y1 = y0 + 1
			}
			for x := 0; x < nw; x++ {
				x0 := b.Min.X + x*sw/nw
				x1 := b.Min.X + (x+1)*sw/nw
				if x1 <= x0 {
					x1 = x0 + 1
				}
				dst.Set(x, y, averageColor(img, x0, y0, x1, y1))
			}
		}
		return dst
	}

	for y := 0; y < nh; y++ {
		sy := b.Min.Y + y*sh/nh
		for x := 0; x < nw; x++ {
			sx := b.Min.X + x*sw/nw
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	return dst
}

// averageColor returns the mean colour of a source rectangle. color.Color.RGBA
// returns alpha-premultiplied components, so averaging them directly is correct
// and avoids dark fringes where the sprite is transparent.
func averageColor(img image.Image, x0, y0, x1, y1 int) color.RGBA64 {
	var sumR, sumG, sumB, sumA, n uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			sumR += uint64(r)
			sumG += uint64(g)
			sumB += uint64(b)
			sumA += uint64(a)
			n++
		}
	}
	if n == 0 {
		return color.RGBA64{}
	}
	return color.RGBA64{
		R: uint16(sumR / n),
		G: uint16(sumG / n),
		B: uint16(sumB / n),
		A: uint16(sumA / n),
	}
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
// Components are un-premultiplied so semi-transparent sprite edges keep their
// colour instead of darkening toward the terminal background.
func rgb(c color.Color) (r, g, b uint8) {
	rr, gg, bb, aa := c.RGBA()
	if aa == 0 {
		return 0, 0, 0
	}
	if aa != 0xffff {
		rr = min32(rr*0xffff/aa, 0xffff)
		gg = min32(gg*0xffff/aa, 0xffff)
		bb = min32(bb*0xffff/aa, 0xffff)
	}
	return uint8(rr >> 8), uint8(gg >> 8), uint8(bb >> 8)
}

func min32(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

// rgbString formats a colour for a 24-bit ANSI sequence.
func rgbString(c color.Color) string {
	r, g, b := rgb(c)
	return fmt.Sprintf("%d;%d;%d", r, g, b)
}
