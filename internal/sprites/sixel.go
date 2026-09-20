package sprites

import (
	"bytes"
	"image"
	"strings"

	sixel "github.com/mattn/go-sixel"
)

// fallbackCellWidth and fallbackCellHeight approximate a terminal cell in
// pixels when the real size cannot be measured.
const (
	fallbackCellWidth  = 10
	fallbackCellHeight = 20
)

// sixelRenderer encodes images as sixel data. Sixel encoding is substantially
// more expensive than the other backends, so callers should avoid re-rendering
// unchanged sprites.
type sixelRenderer struct {
	caps Capabilities
}

func newSixelRenderer(caps Capabilities) *sixelRenderer {
	caps.Protocol = ProtocolSixel
	caps.PixelAccurate = true
	caps.Animated = false
	return &sixelRenderer{caps: caps}
}

func (r *sixelRenderer) Capabilities() Capabilities { return r.caps }

func (r *sixelRenderer) Close() error { return nil }

func (r *sixelRenderer) Render(img image.Image, cols, rows int) (string, error) {
	if img == nil || cols <= 0 || rows <= 0 {
		return "", nil
	}
	cw, ch := r.caps.CellWidth, r.caps.CellHeight
	if cw <= 0 {
		cw = fallbackCellWidth
	}
	if ch <= 0 {
		ch = fallbackCellHeight
	}
	scaled := fit(img, cols*cw, rows*ch)

	var buf bytes.Buffer
	enc := sixel.NewEncoder(&buf)
	if err := enc.Encode(scaled); err != nil {
		return "", err
	}
	out := buf.String()
	if rows > 1 {
		out += strings.Repeat("\n", rows-1)
	}
	return out, nil
}
