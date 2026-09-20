package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/unnipv/pokemon-slowdown/internal/config"
	"github.com/unnipv/pokemon-slowdown/internal/dex"
	"github.com/unnipv/pokemon-slowdown/internal/showdown"
	"github.com/unnipv/pokemon-slowdown/internal/sprites"
	"github.com/unnipv/pokemon-slowdown/internal/storage"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

// TestZZInspectLayouts renders a real mid-battle state at several widths so the
// layout can be eyeballed as text.
func TestLayoutPreview(t *testing.T) {
	if os.Getenv("SLOWDOWN_LIVE") == "" {
		t.Skip("live only (needs dex and sprite cache)")
	}

	d := dex.New(storage.DexDir())
	if err := d.Ensure(context.Background()); err != nil {
		t.Fatalf("dex: %v", err)
	}
	renderer, _ := sprites.NewRenderer(sprites.ModeBlocks, nil)
	mgr := sprites.NewManager(storage.SpriteDir())

	m := New(config.Default(), Deps{Dex: d, Renderer: renderer, Sprites: mgr, Now: time.Now})

	// Replay the recorded battle, stopping mid-way so there are boosts,
	// status, hazards and a populated log.
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "battles", "gen9-singles.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range showdown.SplitFrames(string(raw)) {
		for _, ev := range showdown.Parse(frame) {
			m.handleShowdownEvent(ev)
			if bv := m.activeBattle(); bv != nil && bv.state().Turn >= 4 {
				goto ready
			}
		}
	}
ready:
	bv := m.activeBattle()
	if bv == nil {
		t.Fatal("no battle")
	}

	// Force a full choice request so the move list renders.
	if bv.state().Request == nil || !bv.state().AwaitingChoice {
		t.Log("note: no active choice request; move list may be absent")
	}

	for _, width := range []int{120, 80, 60, 40} {
		m.width, m.height = width, 40
		m.layout = LayoutFor(width)
		out := bv.render(width, 38, m.layout)
		fmt.Printf("\n════════ width %d · %s ════════\n", width, m.layout)
		for _, line := range strings.Split(out, "\n") {
			fmt.Println(stripANSI(line))
		}
	}
}
