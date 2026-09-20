package sprites

import (
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// solid builds a solid-colour RGBA image.
func solid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestParseMode(t *testing.T) {
	cases := map[string]Mode{
		"auto": ModeAuto, "kitty": ModeKitty, "iterm2": ModeITerm2,
		"sixel": ModeSixel, "blocks": ModeBlocks, "none": ModeNone,
		"nonsense": ModeAuto, "": ModeAuto,
	}
	for in, want := range cases {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCandidatesPreferAnimatedThenFallBack(t *testing.T) {
	got := Candidates(Ref{ID: "gengar", Animated: true})
	if len(got) == 0 || !strings.HasSuffix(got[0], "/ani/gengar.gif") {
		t.Fatalf("animated candidates should start with the animated sprite: %#v", got)
	}
	if got[len(got)-1] != BaseURL+"/dex/gengar.png" {
		t.Errorf("final fallback should be dex art, got %q", got[len(got)-1])
	}

	back := Candidates(Ref{ID: "gengar", Back: true, Shiny: true, Animated: true})
	if !strings.Contains(strings.Join(back, " "), "gen5-back-shiny/gengar.png") {
		t.Errorf("shiny back candidates should include the static shiny back: %#v", back)
	}

	// Every candidate list must be duplicate free.
	seen := map[string]bool{}
	for _, u := range back {
		if seen[u] {
			t.Errorf("duplicate candidate %q", u)
		}
		seen[u] = true
	}
}

func TestFitPreservesAspectRatio(t *testing.T) {
	// A wide image fitted into a tall box must be constrained by width.
	src := solid(100, 50, color.RGBA{255, 0, 0, 255})
	got := fit(src, 40, 100)
	if got.Bounds().Dx() != 40 {
		t.Errorf("width = %d, want 40", got.Bounds().Dx())
	}
	if got.Bounds().Dy() != 20 {
		t.Errorf("height = %d, want 20 (aspect preserved)", got.Bounds().Dy())
	}

	// Never produce a zero-sized image.
	tiny := fit(solid(1000, 1000, color.White), 1, 1)
	if tiny.Bounds().Dx() < 1 || tiny.Bounds().Dy() < 1 {
		t.Errorf("degenerate fit: %v", tiny.Bounds())
	}
}

func TestHalfBlockPreservesTransparency(t *testing.T) {
	// Top row opaque red, bottom row transparent.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(1, 0, color.RGBA{255, 0, 0, 255})
	// bottom row left transparent

	r := &halfBlockRenderer{caps: Capabilities{Protocol: ProtocolBlocks}}
	out, err := r.Render(img, 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "\u2580") {
		t.Errorf("expected upper half blocks: %q", out)
	}
	// A fully transparent half must not paint a background colour.
	if strings.Contains(out, "48;2") {
		t.Errorf("transparent lower half should not set a background: %q", out)
	}
	if !strings.HasSuffix(out, "\x1b[0m") {
		t.Errorf("output should reset styling: %q", out)
	}
}

func TestHalfBlockEmitsTwoColoursWhenOpaque(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	img.Set(0, 1, color.RGBA{0, 0, 255, 255})

	r := &halfBlockRenderer{}
	out, _ := r.Render(img, 1, 1)
	if !strings.Contains(out, "38;2;255;0;0") {
		t.Errorf("missing foreground colour: %q", out)
	}
	if !strings.Contains(out, "48;2;0;0;255") {
		t.Errorf("missing background colour: %q", out)
	}
}

func TestHalfBlockGridShape(t *testing.T) {
	img := solid(20, 20, color.RGBA{0, 255, 0, 255})
	r := &halfBlockRenderer{}
	out, _ := r.Render(img, 6, 3)
	lines := strings.Split(strings.TrimSuffix(out, "\x1b[0m"), "\n")
	if len(lines) != 3 {
		t.Fatalf("rows = %d, want 3", len(lines))
	}
	// Each line must contain exactly 6 printable glyphs.
	for i, ln := range lines {
		glyphs := strings.Count(ln, "\u2580") + strings.Count(ln, "\u2584")
		if glyphs != 6 {
			t.Errorf("row %d has %d glyphs, want 6: %q", i, glyphs, ln)
		}
	}
}

func TestNoneRendererIsSilent(t *testing.T) {
	r, err := NewRenderer(ModeNone, color.Black)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(solid(4, 4, color.White), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("none renderer should render nothing, got %q", out)
	}
}

func TestKittyRendererEmitsChunkedProtocol(t *testing.T) {
	r := newKittyRenderer(Detect())
	img := solid(200, 200, color.RGBA{10, 20, 30, 255})
	out, err := r.Render(img, 10, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "\x1b_Gf=100,a=T,c=10,r=5,m=") {
		t.Fatalf("kitty header wrong: %q", out[:min(40, len(out))])
	}
	if !strings.Contains(out, "\x1b\\") {
		t.Error("kitty sequence must be terminated with ST")
	}
	// Multi-row reservation keeps the layout box intact.
	if strings.Count(out, "\n") != 4 {
		t.Errorf("expected 4 reserved newlines, got %d", strings.Count(out, "\n"))
	}
}

// TestKittyChunksLargePayloads exercises the multi-escape path with a noisy
// image that will not compress below one chunk.
func TestKittyChunksLargePayloads(t *testing.T) {
	r := newKittyRenderer(Detect())
	img := noise(240, 240)

	out, err := r.Render(img, 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "\x1b_G"); n < 2 {
		t.Fatalf("expected a chunked payload, got %d escapes", n)
	}
	// Control data belongs to the first chunk only; the rest continue with m=.
	if n := strings.Count(out, "f=100"); n != 1 {
		t.Errorf("control data should appear exactly once, found %d", n)
	}
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "\x1b\\") {
		t.Error("final chunk should be terminated with ST")
	}
	// Continuation chunks must not repeat the control data.
	for _, chunk := range strings.Split(out, "\x1b_G")[1:] {
		if strings.Contains(chunk, "a=T") && !strings.Contains(chunk, "f=100") {
			t.Error("unexpected control data in a continuation chunk")
		}
	}
}

// noise builds a non-compressible RGBA image.
func noise(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	seed := uint32(12345)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			seed = seed*1664525 + 1013904223
			img.Set(x, y, color.RGBA{
				uint8(seed >> 24), uint8(seed >> 16), uint8(seed >> 8), 255,
			})
		}
	}
	return img
}

func TestITerm2Renderer(t *testing.T) {
	r := newITerm2Renderer(Detect())
	out, err := r.Render(solid(20, 20, color.White), 4, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "\x1b]1337;File=inline=1;width=4;height=2;preserveAspectRatio=1:") {
		t.Fatalf("iterm2 header wrong: %q", out)
	}
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "\x07") {
		t.Errorf("iterm2 sequence must end with BEL: %q", out)
	}
}

func TestDecodeSpriteHandlesPNGAndGIF(t *testing.T) {
	pngBytes := encodePNG(t, solid(4, 4, color.RGBA{1, 2, 3, 255}))
	sp, err := decodeSprite(pngBytes)
	if err != nil {
		t.Fatalf("png decode: %v", err)
	}
	if sp.Animated || sp.FrameCount() != 1 {
		t.Errorf("png should be static: %+v", sp)
	}

	// GIF with two frames
	gifBytes := encodeGIF(t)
	sp, err = decodeSprite(gifBytes)
	if err != nil {
		t.Fatalf("gif decode: %v", err)
	}
	if !sp.Animated || sp.FrameCount() != 2 {
		t.Errorf("gif should be animated with 2 frames: %+v", sp)
	}
	if sp.Delay(0) <= 0 {
		t.Error("gif frames should carry a delay")
	}
	if sp.Frame(5) == nil {
		t.Error("Frame should wrap around rather than return nil")
	}
}

func TestManagerCachesAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch {
		case strings.HasSuffix(r.URL.Path, "/ani/gengar.gif"):
			http.NotFound(w, r)
		case strings.HasSuffix(r.URL.Path, "/gen5/gengar.png"):
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(encodePNG(t, solid(8, 8, color.RGBA{200, 0, 200, 255})))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	m := NewManager(dir)
	m.client = srv.Client()
	// Point the resolver at the test server.
	restore := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = restore }()

	ref := Ref{ID: "gengar", Animated: true}
	sp, err := m.Load(context.Background(), ref)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if sp.Static == nil {
		t.Fatal("no image decoded")
	}

	// A second load must be served from memory without another request.
	before := hits
	if _, err := m.Load(context.Background(), ref); err != nil {
		t.Fatalf("second Load: %v", err)
	}
	if hits != before {
		t.Errorf("expected cached load, made %d extra requests", hits-before)
	}
	if _, ok := m.Cached(ref); !ok {
		t.Error("sprite should be cached")
	}

	// A ref that cannot be satisfied is remembered as failed.
	missing := Ref{ID: "notapokemon"}
	if _, err := m.Load(context.Background(), missing); err == nil {
		t.Error("expected an error for a missing sprite")
	}
	if !m.Failed(missing) {
		t.Error("missing sprite should be marked failed")
	}

	stats := m.Stats()
	if stats.Decoded != 1 {
		t.Errorf("stats.Decoded = %d, want 1", stats.Decoded)
	}
}

func TestManagerRejectsUndecodableData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>not an image</html>"))
	}))
	defer srv.Close()

	m := NewManager(t.TempDir())
	m.client = srv.Client()
	restore := BaseURL
	BaseURL = srv.URL
	defer func() { BaseURL = restore }()

	if _, err := m.Load(context.Background(), Ref{ID: "gengar"}); err == nil {
		t.Fatal("expected an error for undecodable data")
	}
}

func TestCleanupEmitsKittyDeleteOnlyForKitty(t *testing.T) {
	var sb strings.Builder
	Cleanup(&sb, Capabilities{Protocol: ProtocolBlocks})
	if sb.Len() != 0 {
		t.Errorf("block renderer needs no cleanup, got %q", sb.String())
	}
	Cleanup(&sb, Capabilities{Protocol: ProtocolKitty})
	if !strings.Contains(sb.String(), "\x1b_Ga=d,d=A") {
		t.Errorf("kitty cleanup should delete images: %q", sb.String())
	}
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "sprite*.png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func encodeGIF(t *testing.T) []byte {
	t.Helper()
	g := &gif.GIF{}
	for i := 0; i < 2; i++ {
		c := color.RGBA{uint8(i * 100), 0, 0, 255}
		g.Image = append(g.Image, imageToPaletted(solid(4, 4, c)))
		g.Delay = append(g.Delay, 10)
	}
	path := filepath.Join(t.TempDir(), "anim.gif")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
	f.Close()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func imageToPaletted(img image.Image) *image.Paletted {
	b := img.Bounds()
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{100, 0, 0, 255}}
	out := image.NewPaletted(b, pal)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}
