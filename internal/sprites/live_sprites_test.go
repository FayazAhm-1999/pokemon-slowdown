package sprites

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// These tests use real sprites downloaded from the public Showdown sprite
// server, because synthetic images cannot catch the failures that matter:
// wrong colour encoding, lost transparency, malformed base64 chunking, or a
// backend that silently renders an empty box.
//
//	SLOWDOWN_LIVE=1 go test ./internal/sprites/ -run TestLive -v

func liveManager(t *testing.T) *Manager {
	t.Helper()
	if os.Getenv("SLOWDOWN_LIVE") == "" {
		t.Skip("set SLOWDOWN_LIVE=1 to run against the live sprite server")
	}
	return NewManager(t.TempDir())
}

func loadLive(t *testing.T, m *Manager, ref Ref) *Sprite {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sp, err := m.Load(ctx, ref)
	if err != nil {
		t.Fatalf("load %s: %v", ref.Key(), err)
	}
	return sp
}

func TestLiveStaticSpritesDecode(t *testing.T) {
	m := liveManager(t)
	cases := []Ref{
		{ID: "gengar"},
		{ID: "gengar", Shiny: true},
		{ID: "gengar", Back: true},
		{ID: "landorus-therian"},
		{ID: "tinglu"},
		{ID: "charizard-megax"},
		{ID: "greattusk"},
		{ID: "meowstic-f"},
	}
	for _, ref := range cases {
		sp := loadLive(t, m, ref)
		if sp.Static == nil {
			t.Fatalf("%s: no static image", ref.Key())
		}
		b := sp.Static.Bounds()
		if b.Dx() < 16 || b.Dy() < 16 {
			t.Errorf("%s: suspiciously small %dx%d", ref.Key(), b.Dx(), b.Dy())
		}
		if sp.URL == "" {
			t.Errorf("%s: no source URL recorded", ref.Key())
		}
		t.Logf("%s -> %s (%dx%d)", ref.Key(), sp.URL, b.Dx(), b.Dy())
	}
}

func TestLiveAnimatedGIFHasFrames(t *testing.T) {
	m := liveManager(t)
	sp := loadLive(t, m, Ref{ID: "gengar", Animated: true})
	if !sp.Animated {
		t.Fatalf("expected an animated sprite, got a single frame from %s", sp.URL)
	}
	if sp.FrameCount() < 2 {
		t.Fatalf("frame count = %d, want at least 2", sp.FrameCount())
	}
	if sp.Delay(0) <= 0 {
		t.Error("animated frames should carry a positive delay")
	}
	t.Logf("gengar.gif: %d frames, first delay %v", sp.FrameCount(), sp.Delay(0))
}

// TestLiveHalfBlockRoundTrip is the important one: it renders a real sprite,
// parses the ANSI back into pixels, and checks the pixels match the source.
// A renderer that emitted a blank or monochrome box would fail here.
func TestLiveHalfBlockRoundTrip(t *testing.T) {
	m := liveManager(t)
	r := &halfBlockRenderer{caps: Capabilities{Protocol: ProtocolBlocks}}

	for _, id := range []string{"gengar", "tinglu", "landorus-therian"} {
		sp := loadLive(t, m, Ref{ID: id})
		cols, rows := 32, 16

		out, err := r.Render(sp.Static, cols, rows)
		if err != nil {
			t.Fatalf("%s: render: %v", id, err)
		}
		if !strings.Contains(out, "\u2580") && !strings.Contains(out, "\u2584") {
			t.Fatalf("%s: no half-block glyphs in output", id)
		}

		got := parseHalfBlocks(t, out, cols, rows)
		want := referencePixels(sp.Static, cols, rows)

		var compared, matched, opaqueCount int
		for row := 0; row < rows; row++ {
			for col := 0; col < cols; col++ {
				for half := 0; half < 2; half++ {
					w := want[row][col][half]
					g := got[row][col][half]
					compared++
					if !opaque(w) {
						continue
					}
					opaqueCount++
					if sameColor(w, g) {
						matched++
					}
				}
			}
		}
		if opaqueCount == 0 {
			t.Fatalf("%s: reference has no opaque pixels", id)
		}
		ratio := float64(matched) / float64(opaqueCount)
		if ratio < 0.98 {
			t.Errorf("%s: only %.1f%% of opaque pixels round-tripped correctly", id, ratio*100)
		}
		t.Logf("%s: %d/%d opaque pixels matched (%.1f%%), %d cells compared",
			id, matched, opaqueCount, ratio*100, compared)

		// Transparency must survive: a sprite has transparent margins.
		if !strings.Contains(out, " ") {
			t.Errorf("%s: expected transparent cells, none found", id)
		}
		// And the sprite must not be a flat block.
		distinct := distinctColors(got)
		if distinct < 4 {
			t.Errorf("%s: only %d distinct colours; the renderer produced a flat box", id, distinct)
		}
		t.Logf("%s: %d distinct rendered colours", id, distinct)
	}
}

func TestLiveKittyPayloadIsValidPNG(t *testing.T) {
	m := liveManager(t)
	r := newKittyRenderer(Detect())

	for _, id := range []string{"gengar", "tinglu"} {
		sp := loadLive(t, m, Ref{ID: id})
		out, err := r.Render(sp.Static, 12, 6)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		payload := extractKittyPayload(t, out)
		img := decodePNG(t, payload)
		src := sp.Static.Bounds()
		if img.Bounds().Dx() != src.Dx() || img.Bounds().Dy() != src.Dy() {
			t.Errorf("%s: kitty payload is %dx%d, source is %dx%d",
				id, img.Bounds().Dx(), img.Bounds().Dy(), src.Dx(), src.Dy())
		}
		if !strings.Contains(out, "c=12") || !strings.Contains(out, "r=6") {
			t.Errorf("%s: kitty control data missing cell dimensions", id)
		}
		if !strings.Contains(out, "f=100") {
			t.Errorf("%s: kitty payload should be PNG (f=100)", id)
		}
		t.Logf("%s: kitty payload %d bytes of base64, decoded to %dx%d",
			id, len(payload), img.Bounds().Dx(), img.Bounds().Dy())
	}
}

func TestLiveITerm2PayloadIsValidPNG(t *testing.T) {
	m := liveManager(t)
	r := newITerm2Renderer(Detect())
	sp := loadLive(t, m, Ref{ID: "gengar"})

	out, err := r.Render(sp.Static, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "\x1b]1337;File=inline=1;width=10;height=5;preserveAspectRatio=1:"
	if !strings.HasPrefix(out, prefix) {
		t.Fatalf("iterm2 header wrong: %q", out[:min(60, len(out))])
	}
	body := strings.TrimPrefix(out, prefix)
	body = strings.TrimRight(body, "\n")
	body = strings.TrimSuffix(body, "\x07")
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		t.Fatalf("iterm2 payload is not valid base64: %v", err)
	}
	img := decodePNG(t, raw)
	src := sp.Static.Bounds()
	if img.Bounds().Dx() != src.Dx() {
		t.Errorf("iterm2 payload width %d, source %d", img.Bounds().Dx(), src.Dx())
	}
}

func TestLiveSixelPayload(t *testing.T) {
	m := liveManager(t)
	r := newSixelRenderer(Detect())
	sp := loadLive(t, m, Ref{ID: "gengar"})

	out, err := r.Render(sp.Static, 12, 6)
	if err != nil {
		t.Fatalf("sixel encode: %v", err)
	}
	if !strings.HasPrefix(out, "\x1bP") {
		t.Fatalf("sixel data should start with DCS: %q", out[:min(20, len(out))])
	}
	if !strings.Contains(out, "q") {
		t.Error("sixel data missing the raster attribute introducer")
	}
	// Sixel is the expensive backend; make sure we actually produced data.
	if len(out) < 500 {
		t.Errorf("sixel output is suspiciously short: %d bytes", len(out))
	}
	t.Logf("gengar sixel: %d bytes", len(out))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var csiRGB = regexp.MustCompile(`^\x1b\[(38|48);2;(\d+);(\d+);(\d+)m`)

// parseHalfBlocks decodes the block renderer's ANSI back into per-cell pixels.
func parseHalfBlocks(t *testing.T, s string, cols, rows int) [][][2]color.Color {
	t.Helper()
	grid := make([][][2]color.Color, rows)
	for i := range grid {
		grid[i] = make([][2]color.Color, cols)
	}

	var fg, bg color.Color
	col, row := 0, 0
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			if m := csiRGB.FindStringSubmatch(s[i:]); m != nil {
				r, _ := strconv.Atoi(m[2])
				g, _ := strconv.Atoi(m[3])
				b, _ := strconv.Atoi(m[4])
				c := color.RGBA{uint8(r), uint8(g), uint8(b), 255}
				if m[1] == "38" {
					fg = c
				} else {
					bg = c
				}
				i += len(m[0])
				continue
			}
			if strings.HasPrefix(s[i:], "\x1b[49m") {
				bg = nil
				i += len("\x1b[49m")
				continue
			}
			if strings.HasPrefix(s[i:], "\x1b[0m") {
				fg, bg = nil, nil
				i += len("\x1b[0m")
				continue
			}
			i++
			continue
		}

		switch s[i] {
		case '\n':
			row++
			col = 0
		case ' ':
			col++
		case 0xe2: // UTF-8 lead byte for the block glyphs
			_, size := decodeRune(s[i:])
			if row < rows && col < cols {
				if strings.HasPrefix(s[i:], "\u2580") {
					grid[row][col] = [2]color.Color{fg, bg}
				} else {
					grid[row][col] = [2]color.Color{bg, fg}
				}
			}
			col++
			i += size
			continue
		default:
			col++
		}
		i++
	}
	return grid
}

func decodeRune(s string) (rune, int) {
	for _, r := range s {
		n := len(string(r))
		return r, n
	}
	return 0, 1
}

// referencePixels reproduces what the renderer should have drawn, using the
// same fit and centring the renderer uses.
func referencePixels(img image.Image, cols, rows int) [][][2]color.Color {
	scaled := fit(img, cols, rows*2)
	b := scaled.Bounds()
	offX := (cols - b.Dx()) / 2
	offY := (rows*2 - b.Dy()) / 2

	grid := make([][][2]color.Color, rows)
	for row := 0; row < rows; row++ {
		grid[row] = make([][2]color.Color, cols)
		for col := 0; col < cols; col++ {
			grid[row][col] = [2]color.Color{
				pixelAt(scaled, col-offX, row*2-offY),
				pixelAt(scaled, col-offX, row*2+1-offY),
			}
		}
	}
	return grid
}

// sameColor compares two colours as they are actually rendered: the renderer
// un-premultiplies before emitting, so the reference must too.
func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return opaque(a) == opaque(b)
	}
	ar, ag, ab, aa := straight(a)
	br, bg, bb, ba := straight(b)
	if (aa == 0) != (ba == 0) {
		return false
	}
	return ar == br && ag == bg && ab == bb
}

// straight returns colour components as they are actually emitted: the
// renderer un-premultiplies and quantises to 8 bits per channel, so the
// reference must be quantised the same way to compare equal.
func straight(c color.Color) (r, g, b, a uint32) {
	r, g, b, a = c.RGBA()
	if a == 0 {
		return 0, 0, 0, 0
	}
	if a != 0xffff {
		r = r * 0xffff / a
		g = g * 0xffff / a
		b = b * 0xffff / a
	}
	return quant8(r), quant8(g), quant8(b), 0xffff
}

// quant8 reduces a 16-bit channel to the 8-bit value the terminal receives,
// then widens it back so comparisons are like for like.
func quant8(v uint32) uint32 {
	if v > 0xffff {
		v = 0xffff
	}
	return (v >> 8) * 0x101
}

func distinctColors(grid [][][2]color.Color) int {
	seen := map[uint32]bool{}
	for _, row := range grid {
		for _, cell := range row {
			for _, c := range cell {
				if !opaque(c) {
					continue
				}
				r, g, b, _ := c.RGBA()
				seen[uint32(r>>8)<<16|uint32(g>>8)<<8|uint32(b>>8)] = true
			}
		}
	}
	return len(seen)
}

func extractKittyPayload(t *testing.T, out string) []byte {
	t.Helper()
	var b64 strings.Builder
	rest := out
	for {
		i := strings.Index(rest, "\x1b_G")
		if i < 0 {
			break
		}
		rest = rest[i+len("\x1b_G"):]
		j := strings.Index(rest, "\x1b\\")
		if j < 0 {
			t.Fatal("unterminated kitty escape")
		}
		chunk := rest[:j]
		rest = rest[j+len("\x1b\\"):]
		semi := strings.IndexByte(chunk, ';')
		if semi < 0 {
			continue
		}
		b64.WriteString(chunk[semi+1:])
	}
	raw, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil {
		t.Fatalf("kitty base64 did not decode: %v", err)
	}
	return raw
}

func decodePNG(t *testing.T, raw []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("payload is not a valid PNG: %v", err)
	}
	return img
}
