package tui

import (
	"hash/fnv"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/unnipv/pokemon-slowdown/internal/battle"
	"github.com/unnipv/pokemon-slowdown/internal/sprites"
)

// Bubble Tea parses view content into a cell buffer and discards escape
// sequences it does not recognise, so a Kitty graphics escape embedded in a
// View never reaches the terminal.
//
// The way through is to put a *printable sentinel rune* in the sprite's
// top-left cell, let Bubble Tea lay it out and write it like any other
// character, and then substitute that rune for the graphics payload in the
// writer on the way out. Positioning is handled by Bubble Tea itself, so no
// cursor arithmetic is needed.
//
// Sentinels are private-use runes chosen from a hash of the payload, so a cell
// is rewritten exactly when its sprite changes, and unchanged sprites cost
// nothing.
const (
	sentinelBase  = 0xE000
	sentinelRange = 0x800
)

// spriteLayer tracks the graphics payloads for the current frame and rewrites
// sentinels on the way to the terminal.
type spriteLayer struct {
	mu       sync.Mutex
	payloads map[rune]string
	ids      map[string]uint32
	nextID   uint32

	// gen changes whenever the set of rendered sprites changes, which forces
	// every sentinel to change so all sprites are rewritten after a clear.
	gen     int
	lastSet string
	key     int
	drawn   int
}

func newSpriteLayer() *spriteLayer {
	return &spriteLayer{
		payloads: map[rune]string{},
		ids:      map[string]uint32{},
		nextID:   1,
	}
}

// begin starts a frame. setKey identifies which sprites are on screen; when it
// changes, all sentinels change so the whole layer is redrawn.
func (l *spriteLayer) begin(setKey string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if setKey != l.lastSet {
		l.gen++
		l.lastSet = setKey
		l.key = l.gen
	}
	l.payloads = make(map[rune]string, 8)
}

// ID returns a stable image id for a sprite slot, so a replacement can delete
// the previous image at that slot.
func (l *spriteLayer) ID(slot string) uint32 {
	l.mu.Lock()
	defer l.mu.Unlock()
	if id, ok := l.ids[slot]; ok {
		return id
	}
	id := l.nextID
	l.nextID++
	if l.nextID > 64 {
		l.nextID = 1
	}
	l.ids[slot] = id
	return id
}

// register stores a payload for this frame and returns the rune to place in the
// sprite's top-left cell.
func (l *spriteLayer) register(payload string) rune {
	l.mu.Lock()
	defer l.mu.Unlock()
	h := fnv.New32a()
	_, _ = h.Write([]byte(payload))
	base := rune(sentinelBase + (h.Sum32()^uint32(l.gen))%sentinelRange)
	for i := 0; i < sentinelRange; i++ {
		r := rune(sentinelBase + (int(base-sentinelBase)+i)%sentinelRange)
		if existing, ok := l.payloads[r]; !ok || existing == payload {
			l.payloads[r] = payload
			return r
		}
	}
	// The range is exhausted; fall back to the base rune.
	l.payloads[base] = payload
	return base
}

// wrap returns a writer that forwards to out but substitutes sentinels for
// graphics payloads, clearing the layer first when the sprite set changed.
func (l *spriteLayer) wrap(out io.Writer) io.Writer {
	return &layerWriter{out: out, layer: l}
}

type layerWriter struct {
	out   io.Writer
	layer *spriteLayer
}

// Fd forwards the underlying file descriptor. Bubble Tea uses it to decide
// whether it is talking to a terminal.
func (w *layerWriter) Fd() uintptr {
	if f, ok := w.out.(interface{ Fd() uintptr }); ok {
		return f.Fd()
	}
	return ^uintptr(0)
}

// Read and Close exist so the wrapper satisfies term.File (io.ReadWriteCloser
// plus Fd). Bubble Tea only treats the output as a TTY - and therefore only
// asks it for a size - when that assertion succeeds. Without these the program
// renders nothing at all.
func (w *layerWriter) Read(p []byte) (int, error) {
	if r, ok := w.out.(io.Reader); ok {
		return r.Read(p)
	}
	return 0, io.EOF
}

// Close is deliberately a no-op: this wraps the terminal, which the program
// must not close on the way out.
func (w *layerWriter) Close() error { return nil }

func (w *layerWriter) Write(p []byte) (int, error) {
	w.layer.mu.Lock()
	payloads := w.layer.payloads
	clear := w.layer.key != w.layer.drawn
	if clear {
		w.layer.drawn = w.layer.key
	}
	w.layer.mu.Unlock()

	body := substituteSentinels(p, payloads)
	if clear {
		body = append([]byte(sprites.KittyDeleteAll()), body...)
	}
	if len(body) == 0 {
		return len(p), nil
	}
	if _, err := w.out.Write(body); err != nil {
		return 0, err
	}
	// The caller's contract is about the bytes it handed us, not the bytes we
	// synthesised.
	return len(p), nil
}

// substituteSentinels replaces private-use sentinel runes with their payloads.
func substituteSentinels(p []byte, payloads map[rune]string) []byte {
	if len(payloads) == 0 || len(p) == 0 {
		return p
	}
	// Private use area U+E000..U+EFFF always starts with the lead byte 0xEE,
	// so a chunk with none of those bytes cannot contain a sentinel.
	if !hasSentinelLead(p) {
		return p
	}

	out := make([]byte, 0, len(p)+256)
	for i := 0; i < len(p); {
		r, size := utf8.DecodeRune(p[i:])
		if r >= sentinelBase && r < sentinelBase+sentinelRange {
			if payload, ok := payloads[r]; ok {
				out = append(out, payload...)
				i += size
				continue
			}
		}
		out = append(out, p[i:i+size]...)
		i += size
	}
	return out
}

func hasSentinelLead(p []byte) bool {
	for _, b := range p {
		if b == 0xEE {
			return true
		}
	}
	return false
}

// spriteSetKey identifies the sprites on screen. It is stable while the same
// Pokémon are out, so unchanged sprites are not re-sent.
func (bv *battleView) spriteSetKey() string {
	s := bv.state()
	animate := bv.animateSprites()
	var parts []string
	add := func(side *battle.Side, back bool) {
		if side == nil {
			return
		}
		for i, p := range side.ActiveParty() {
			parts = append(parts, side.ID+"/"+strconv.Itoa(i)+"/"+bv.spriteRef(p, back, animate).Key())
		}
	}
	add(s.Opponent(), false)
	add(s.MySide(), true)
	for _, p := range s.P1.Party {
		parts = append(parts, "p1/"+p.Ident)
	}
	for _, p := range s.P2.Party {
		parts = append(parts, "p2/"+p.Ident)
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

// layerKey includes the layout and overlay state, so opening an overlay or
// switching to a layout without sprites invalidates the whole layer.
func (bv *battleView) layerKey(layout LayoutMode) string {
	key := bv.spriteSetKey() + "|" + layout.String()
	if bv.overlay != overlayNone {
		key += "|overlay"
	}
	return key
}

// spriteSlot names a sprite's position so it keeps a stable image id.
func (bv *battleView) spriteSlot(p *battle.Pokemon, back bool) string {
	if p == nil {
		return ""
	}
	if p.Slot != "" {
		return p.SideID + p.Slot
	}
	return p.SideID + ":" + p.Ident
}
