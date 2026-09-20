package sprites

import (
	"image"
	"image/color"
	"strings"
)

// halfBlockRenderer renders sprites as Unicode upper-half blocks. Each
// character cell carries two vertically stacked pixels, using the foreground
// colour for the top pixel and the background colour for the bottom pixel.
//
// Transparency is preserved rather than painted over: when only one half of a
// cell is opaque the other half is left showing the terminal's own background.
type halfBlockRenderer struct {
	caps Capabilities
	bg   color.Color
}

func (r *halfBlockRenderer) Capabilities() Capabilities { return r.caps }

func (r *halfBlockRenderer) Close() error { return nil }

func (r *halfBlockRenderer) Render(img image.Image, cols, rows int) (string, error) {
	if img == nil || cols <= 0 || rows <= 0 {
		return "", nil
	}

	pxW, pxH := cols, rows*2
	scaled := fit(img, pxW, pxH)
	b := scaled.Bounds()
	offX := (pxW - b.Dx()) / 2
	offY := (pxH - b.Dy()) / 2

	var sb strings.Builder
	sb.Grow(cols * rows * 24)

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			top := pixelAt(scaled, col-offX, row*2-offY)
			bottom := pixelAt(scaled, col-offX, row*2+1-offY)
			writeCell(&sb, top, bottom)
		}
		if row < rows-1 {
			sb.WriteByte('\n')
		}
	}
	sb.WriteString("\x1b[0m")
	return sb.String(), nil
}

// pixelAt returns the pixel at x,y or a fully transparent colour when the
// coordinate falls outside the image, so sprites are centred in their box.
func pixelAt(img *image.RGBA, x, y int) color.Color {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return color.Transparent
	}
	return img.At(x, y)
}

// writeCell emits one character cell. The escape sequences are only as verbose
// as they need to be: a cell with a transparent half never sets a background.
func writeCell(sb *strings.Builder, top, bottom color.Color) {
	topOn := opaque(top)
	bottomOn := opaque(bottom)

	switch {
	case !topOn && !bottomOn:
		sb.WriteByte(' ')
	case topOn && bottomOn:
		sb.WriteString("\x1b[38;2;")
		sb.WriteString(rgbString(top))
		sb.WriteString("m\x1b[48;2;")
		sb.WriteString(rgbString(bottom))
		sb.WriteString("m\u2580") // ▀ upper half block
	case topOn:
		sb.WriteString("\x1b[38;2;")
		sb.WriteString(rgbString(top))
		sb.WriteString("m\x1b[49m\u2580")
	default:
		sb.WriteString("\x1b[38;2;")
		sb.WriteString(rgbString(bottom))
		sb.WriteString("m\x1b[49m\u2584") // ▄ lower half block
	}
}
