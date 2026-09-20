package sprites

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
)

// iterm2Renderer places images with the iTerm2 inline image protocol, which is
// also understood by WezTerm and a number of other terminals.
type iterm2Renderer struct {
	caps Capabilities
}

func newITerm2Renderer(caps Capabilities) *iterm2Renderer {
	caps.Protocol = ProtocolITerm2
	caps.PixelAccurate = true
	caps.Animated = false
	return &iterm2Renderer{caps: caps}
}

func (r *iterm2Renderer) Capabilities() Capabilities { return r.caps }

func (r *iterm2Renderer) Close() error { return nil }

func (r *iterm2Renderer) Render(img image.Image, cols, rows int) (string, error) {
	if img == nil || cols <= 0 || rows <= 0 {
		return "", nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	var sb strings.Builder
	fmt.Fprintf(&sb, "\x1b]1337;File=inline=1;width=%d;height=%d;preserveAspectRatio=1:", cols, rows)
	sb.WriteString(b64)
	sb.WriteString("\x07")
	if rows > 1 {
		sb.WriteString(strings.Repeat("\n", rows-1))
	}
	return sb.String(), nil
}
