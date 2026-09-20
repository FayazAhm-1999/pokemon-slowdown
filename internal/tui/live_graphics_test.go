package tui

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/unnipv/pokemon-slowdown/internal/config"
	"github.com/unnipv/pokemon-slowdown/internal/dex"
	"github.com/unnipv/pokemon-slowdown/internal/showdown"
	"github.com/unnipv/pokemon-slowdown/internal/sprites"
	"github.com/unnipv/pokemon-slowdown/internal/storage"
)

// TestLiveKittyReachesTheTerminal is the end-to-end proof for out-of-band
// sprites: it runs a real Bubble Tea program with the real battle model and a
// real sprite, and inspects the bytes that would have been written to the
// terminal.
//
//	SLOWDOWN_LIVE=1 go test ./internal/tui/ -run TestLiveKittyReachesTheTerminal -v
func TestLiveKittyReachesTheTerminal(t *testing.T) {
	if os.Getenv("SLOWDOWN_LIVE") == "" {
		t.Skip("live only (needs the dex and sprite server)")
	}

	renderer, err := sprites.NewRenderer(sprites.ModeKitty, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !sprites.SupportsPayload(renderer) {
		t.Fatal("kitty renderer does not support payloads")
	}

	mgr := sprites.NewManager(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d := dex.New(storage.DexDir())
	_ = d.Ensure(ctx)

	m := New(config.Default(), Deps{
		Dex:      d,
		Sprites:  mgr,
		Renderer: renderer,
		Now:      time.Now,
	})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "battles", "gen9-singles.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range showdown.SplitFrames(string(raw)) {
		for _, ev := range showdown.Parse(frame) {
			m.handleShowdownEvent(ev)
			if bv := m.activeBattle(); bv != nil && bv.state().Turn >= 1 {
				goto ready
			}
		}
	}
ready:

	// Preload exactly the sprites the view asks for (own Pokémon use back
	// sprites), so the test exercises the payload path rather than the
	// placeholder.
	if bv := m.activeBattle(); bv != nil {
		wanted := bv.wantedSprites(false)
		if len(wanted) == 0 {
			t.Fatal("view wants no sprites; nothing to test")
		}
		for _, req := range wanted {
			if _, err := mgr.Load(ctx, req.Sprite); err != nil {
				t.Fatalf("preload %s: %v", req.Key, err)
			}
			t.Logf("preloaded %s -> %s", req.Key, req.Sprite.ID)
		}
	}

	var buf bytes.Buffer
	pr, pw := io.Pipe()
	defer pw.Close()
	defer pr.Close()

	pctx, pcancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer pcancel()

	p := tea.NewProgram(
		m,
		tea.WithOutput(m.SpriteWriter(&buf)),
		tea.WithInput(pr),
		tea.WithContext(pctx),
		tea.WithWindowSize(120, 40),
	)
	_, _ = p.Run()

	out := buf.String()
	t.Logf("terminal received %d bytes", len(out))

	if !strings.Contains(out, "\x1b_G") {
		preview := out
		if len(preview) > 400 {
			preview = preview[:400]
		}
		t.Fatalf("no kitty graphics escape reached the terminal; first 400 bytes:\n%q", preview)
	}
	if !strings.Contains(out, "iVBORw0KGgo") {
		t.Error("kitty escape reached the terminal but carried no PNG payload")
	}
	if !strings.Contains(out, "a=T") {
		t.Error("payload is missing the display action")
	}
	if !strings.Contains(out, "C=1") {
		t.Error("payload must not move the cursor (C=1), or it would corrupt the layout")
	}
	// The sentinel must have been consumed, never printed.
	for r := rune(sentinelBase); r < sentinelBase+sentinelRange; r++ {
		if strings.ContainsRune(out, r) {
			t.Fatalf("sentinel rune %U was written to the terminal", r)
		}
	}
	// And the reserved rectangle must still be laid out.
	if !strings.Contains(out, "Alcremie") && !strings.Contains(out, "Gengar") {
		t.Log("note: expected a Pokémon name in the frame")
	}
	t.Log("kitty payload reached the terminal intact")
}
