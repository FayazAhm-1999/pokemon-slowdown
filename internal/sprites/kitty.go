package sprites

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"strings"
)

// Kitty graphics protocol limits a single escape payload to 4096 base64 bytes.
const kittyChunkSize = 4096

// kittyRenderer places images using the Kitty graphics protocol. It is opt-in:
// the block renderer is the reliable default inside a full-screen TUI.
type kittyRenderer struct {
	caps Capabilities
}

func newKittyRenderer(caps Capabilities) *kittyRenderer {
	caps.Protocol = ProtocolKitty
	caps.PixelAccurate = true
	caps.Animated = true
	return &kittyRenderer{caps: caps}
}

func (r *kittyRenderer) Capabilities() Capabilities { return r.caps }

func (r *kittyRenderer) Close() error { return nil }

// Render transmits a PNG and displays it in a cols x rows cell rectangle. The
// terminal scales the image to the box, so no client-side scaling is needed.
func (r *kittyRenderer) Render(img image.Image, cols, rows int) (string, error) {
	if img == nil || cols <= 0 || rows <= 0 {
		return "", nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	var sb strings.Builder
	for i := 0; i < len(b64); i += kittyChunkSize {
		end := i + kittyChunkSize
		if end > len(b64) {
			end = len(b64)
		}
		more := 0
		if end < len(b64) {
			more = 1
		}
		if i == 0 {
			fmt.Fprintf(&sb, "\x1b_Gf=100,a=T,c=%d,r=%d,m=%d;%s\x1b\\", cols, rows, more, b64[i:end])
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", more, b64[i:end])
		}
	}
	// Reserve the rectangle's remaining rows so the layout stays intact.
	if rows > 1 {
		sb.WriteString(strings.Repeat("\n", rows-1))
	}
	return sb.String(), nil
}

// kittyDeleteAll removes every image the terminal has been asked to display.
const kittyDeleteAll = "\x1b_Ga=d,d=A\x1b\\"

// KittyDeleteAll returns the sequence that removes every placed image.
func KittyDeleteAll() string { return kittyDeleteAll }

// KittyDeleteImage returns the sequence that removes one image by id.
func KittyDeleteImage(id uint32) string {
	return fmt.Sprintf("\x1b_Ga=d,d=I,i=%d\x1b\\", id)
}

// Payload encodes a PNG transmission that displays the image at the current
// cursor position in a cols x rows cell box, tagged with an image id.
//
// C=1 stops the terminal moving the cursor, which is what makes this safe to
// splice into a rendered frame. The payload carries no trailing newlines: it is
// drawn out of band next to a reserved blank rectangle, not pasted into the
// view.
func (r *kittyRenderer) Payload(img image.Image, cols, rows int, id uint32) (string, error) {
	if img == nil || cols <= 0 || rows <= 0 {
		return "", nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	var sb strings.Builder
	for i := 0; i < len(b64); i += kittyChunkSize {
		end := i + kittyChunkSize
		if end > len(b64) {
			end = len(b64)
		}
		more := 0
		if end < len(b64) {
			more = 1
		}
		if i == 0 {
			fmt.Fprintf(&sb, "\x1b_Gf=100,a=T,C=1,i=%d,c=%d,r=%d,m=%d;%s\x1b\\", id, cols, rows, more, b64[i:end])
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d;%s\x1b\\", more, b64[i:end])
		}
	}
	return sb.String(), nil
}

// SupportsPayload reports whether a renderer can draw out of band (that is,
// with a payload injected into the output stream rather than inline text).
func SupportsPayload(r Renderer) bool {
	_, ok := r.(interface {
		Payload(image.Image, int, int, uint32) (string, error)
	})
	return ok
}

// PayloadFor returns an out-of-band graphics payload if the renderer supports
// one, otherwise false. Only pixel backends implement it: the block renderer
// draws inline and needs no payload.
func PayloadFor(r Renderer, img image.Image, cols, rows int, id uint32) (string, bool) {
	pr, ok := r.(interface {
		Payload(image.Image, int, int, uint32) (string, error)
	})
	if !ok {
		return "", false
	}
	out, err := pr.Payload(img, cols, rows, id)
	if err != nil {
		return "", false
	}
	return out, true
}

// Cleanup writes whatever escape sequences are required to remove images the
// given capabilities imply, so quitting never leaves sprite remnants behind.
func Cleanup(w io.Writer, caps Capabilities) {
	if w == nil {
		return
	}
	if caps.Protocol == ProtocolKitty {
		_, _ = io.WriteString(w, kittyDeleteAll)
	}
}
