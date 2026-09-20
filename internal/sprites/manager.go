package sprites

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// maxSpriteBytes caps a downloaded sprite.
const maxSpriteBytes = 4 << 20

// Sprite is a decoded sprite, possibly animated.
type Sprite struct {
	Frames []image.Image
	Delays []time.Duration
	Static image.Image
	// URL records where the sprite was loaded from.
	URL string
	// Animated reports whether there is more than one frame.
	Animated bool
}

// FrameCount returns the number of animation frames.
func (s *Sprite) FrameCount() int {
	if s == nil {
		return 0
	}
	return len(s.Frames)
}

// Frame returns frame i, or the static image when out of range.
func (s *Sprite) Frame(i int) image.Image {
	if s == nil || len(s.Frames) == 0 {
		return nil
	}
	return s.Frames[i%len(s.Frames)]
}

// Delay returns the display duration of frame i.
func (s *Sprite) Delay(i int) time.Duration {
	if s == nil || len(s.Delays) == 0 {
		return 0
	}
	d := s.Delays[i%len(s.Delays)]
	if d <= 0 {
		return 100 * time.Millisecond
	}
	return d
}

// Manager downloads, decodes and caches sprites. Loading always happens off the
// UI path: Cached is instant and Load is meant to be called from a command.
type Manager struct {
	dir    string
	client *http.Client

	mu    sync.Mutex
	cache map[string]*Sprite
	// failed records refs whose every candidate 404ed, so the UI can stop
	// asking and show a placeholder.
	failed map[string]bool
}

// NewManager returns a sprite manager caching files in dir.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:    dir,
		client: &http.Client{Timeout: 30 * time.Second},
		cache:  map[string]*Sprite{},
		failed: map[string]bool{},
	}
}

// Dir returns the sprite cache directory.
func (m *Manager) Dir() string { return m.dir }

// Cached returns an already-decoded sprite without touching the network.
func (m *Manager) Cached(ref Ref) (*Sprite, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.cache[ref.Key()]
	return s, ok
}

// Failed reports whether every candidate for a ref has already 404ed.
func (m *Manager) Failed(ref Ref) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failed[ref.Key()]
}

// Load returns a decoded sprite, downloading it if needed. It is safe to call
// concurrently; duplicate work is suppressed.
func (m *Manager) Load(ctx context.Context, ref Ref) (*Sprite, error) {
	key := ref.Key()
	if s, ok := m.Cached(ref); ok {
		return s, nil
	}
	if m.Failed(ref) {
		return nil, fmt.Errorf("sprites: no sprite published for %s", ref.ID)
	}

	var lastErr error
	for _, url := range Candidates(ref) {
		raw, err := m.fetch(ctx, url)
		if err != nil {
			lastErr = err
			continue
		}
		sprite, err := decodeSprite(raw)
		if err != nil {
			lastErr = err
			continue
		}
		sprite.URL = url
		m.mu.Lock()
		m.cache[key] = sprite
		m.mu.Unlock()
		return sprite, nil
	}

	m.mu.Lock()
	m.failed[key] = true
	m.mu.Unlock()
	if lastErr == nil {
		lastErr = fmt.Errorf("sprites: no candidates for %s", ref.ID)
	}
	return nil, lastErr
}

// fetch returns sprite bytes, preferring the on-disk cache.
func (m *Manager) fetch(ctx context.Context, url string) ([]byte, error) {
	path := m.cachePath(url)
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		return raw, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sprites: %s: %s", url, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxSpriteBytes))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err == nil {
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, raw, 0o644); err == nil {
			_ = os.Rename(tmp, path)
		}
	}
	return raw, nil
}

// cachePath maps a sprite URL to a readable path inside the cache directory.
func (m *Manager) cachePath(url string) string {
	rel := strings.TrimPrefix(url, BaseURL+"/")
	rel = strings.ReplaceAll(rel, "/", "_")
	if rel == "" {
		rel = "sprite"
	}
	return filepath.Join(m.dir, rel)
}

// decodeSprite decodes PNG or GIF bytes into a Sprite.
func decodeSprite(raw []byte) (*Sprite, error) {
	if bytes.HasPrefix(raw, []byte("GIF8")) {
		g, err := gif.DecodeAll(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		if len(g.Image) == 0 {
			return nil, fmt.Errorf("sprites: empty GIF")
		}
		sp := &Sprite{}
		for i, frame := range g.Image {
			sp.Frames = append(sp.Frames, frame)
			d := 100 * time.Millisecond
			if i < len(g.Delay) && g.Delay[i] > 0 {
				d = time.Duration(g.Delay[i]) * 10 * time.Millisecond
			}
			sp.Delays = append(sp.Delays, d)
		}
		sp.Static = sp.Frames[0]
		sp.Animated = len(sp.Frames) > 1
		return sp, nil
	}

	if img, err := png.Decode(bytes.NewReader(raw)); err == nil {
		return &Sprite{Frames: []image.Image{img}, Static: img, Delays: []time.Duration{0}}, nil
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("sprites: undecodable image: %w", err)
	}
	return &Sprite{Frames: []image.Image{img}, Static: img, Delays: []time.Duration{0}}, nil
}

// Stats reports cache counts for the doctor command.
type Stats struct {
	Decoded int
	Failed  int
	Dir     string
}

// Stats returns cache statistics.
func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Stats{Decoded: len(m.cache), Failed: len(m.failed), Dir: m.dir}
}
