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
