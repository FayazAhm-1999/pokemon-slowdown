package tui

import (
	"bytes"
	"github.com/unnipv/pokemon-slowdown/internal/config"
	"strings"
	"testing"
)

func TestSpriteLayerSubstitutesSentinels(t *testing.T) {
	l := newSpriteLayer()
	var buf bytes.Buffer
	w := l.wrap(&buf)

	l.begin("set1")
	r := l.register("\x1b_GPAYLOAD\x1b\\")
	if _, err := w.Write([]byte("hello " + string(r) + " world\n")); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "\x1b_GPAYLOAD") {
		t.Errorf("payload was not substituted: %q", out)
	}
	if strings.ContainsRune(out, r) {
		t.Error("sentinel rune leaked to the terminal")
	}
	if !strings.Contains(out, "hello ") || !strings.Contains(out, " world") {
		t.Errorf("surrounding frame text was lost: %q", out)
	}
}

func TestSpriteLayerClearsOnlyWhenTheSetChanges(t *testing.T) {
	l := newSpriteLayer()
	var buf bytes.Buffer
	w := l.wrap(&buf)

	l.begin("set1")
	r := l.register("\x1b_GONE\x1b\\")
	_, _ = w.Write([]byte(string(r)))
	buf.Reset()

	// Same set: no clear, sprite cells unchanged so nothing is re-sent.
	l.begin("set1")
	r2 := l.register("\x1b_GONE\x1b\\")
	_, _ = w.Write([]byte("x" + string(r2) + "y"))
	if strings.Contains(buf.String(), "\x1b_Ga=d,d=A") {
		t.Errorf("unexpected delete-all for an unchanged sprite set: %q", buf.String())
	}

	// A different set must clear the old layer before drawing.
	buf.Reset()
	l.begin("set2")
	r3 := l.register("\x1b_GTWO\x1b\\")
	_, _ = w.Write([]byte(string(r3)))
	out := buf.String()
	if !strings.Contains(out, "\x1b_Ga=d,d=A") {
		t.Errorf("expected delete-all when the sprite set changed: %q", out)
	}
	if !strings.Contains(out, "\x1b_GTWO") {
		t.Errorf("new payload missing: %q", out)
	}
}

func TestSpriteLayerPayloadChangeAltersTheSentinel(t *testing.T) {
	l := newSpriteLayer()
	l.begin("set")
	first := l.register("\x1b_GA\x1b\\")
	l.begin("set")
	second := l.register("\x1b_GB\x1b\\")
	if first == second {
		t.Error("a changed payload must produce a different sentinel, or the cell is never rewritten")
	}
}

func TestSpriteLayerIDsAreStablePerSlot(t *testing.T) {
	l := newSpriteLayer()
	a1 := l.ID("p1a")
	a2 := l.ID("p1a")
	b := l.ID("p2a")
	if a1 != a2 {
		t.Errorf("slot id changed: %d then %d", a1, a2)
	}
	if a1 == b {
		t.Error("different slots must have different image ids")
	}
}

func TestSubstituteSentinelsLeavesOrdinaryTextAlone(t *testing.T) {
	in := []byte("plain ascii text with a pipe | and a block ▀")
	got := substituteSentinels(in, map[rune]string{0xE000: "X"})
	if !bytes.Equal(got, in) {
		t.Errorf("ordinary text was modified: %q -> %q", in, got)
	}
}

func TestOverlayInvalidatesTheSpriteLayer(t *testing.T) {
	m := testModel(t, config.Default())
	bv := m.battleFor("battle-x")

	open := bv.layerKey(LayoutStandard)
	bv.overlay = overlayInspect
	withOverlay := bv.layerKey(LayoutStandard)

	if open == withOverlay {
		t.Error("opening an overlay must invalidate the sprite layer, or sprites draw over it")
	}
	// Changing layout or returning to no overlay must invalidate again.
	bv.overlay = overlayNone
	if back := bv.layerKey(LayoutStandard); back != open {
		t.Error("closing the overlay should restore the original key")
	}
	if other := bv.layerKey(LayoutCompact); other == open {
		t.Error("changing layout must invalidate the sprite layer")
	}
}
