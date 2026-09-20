// Package dex provides the small amount of Pokémon Showdown reference data the
// client needs at runtime: species types and sprite identifiers, and move
// types, categories and power. The data is fetched once from the public
// Showdown data endpoints and cached on disk.
package dex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// BaseURL is where Showdown publishes its dex data.
const BaseURL = "https://play.pokemonshowdown.com/data"

// maxDexBytes caps each downloaded data file.
const maxDexBytes = 16 << 20

// Species is the subset of a pokedex entry the client uses.
type Species struct {
	Key         string
	Name        string
	Num         int
	Types       []string
	BaseStats   map[string]int
	Abilities   map[string]string
	BaseSpecies string
	Forme       string
	Gen         int
	Tier        string
}

// SpriteID returns the sprite filename stem for this species. Showdown names
// formes as "<basespecies>-<forme>" using toID on both halves, and everything
// else as the plain toID of the species name.
func (s Species) SpriteID() string {
	if s.BaseSpecies != "" && s.Forme != "" {
		return ToID(s.BaseSpecies) + "-" + ToID(s.Forme)
	}
	return ToID(s.Name)
}

// Move is the subset of a move entry the client uses.
type Move struct {
	ID        string
	Name      string
	Type      string
	Category  string
	PP        int
	BasePower int
	Target    string
	ShortDesc string
}

// Dex is a lazily loaded reference catalogue.
type Dex struct {
	dir    string
	client *http.Client

	mu      sync.RWMutex
	species map[string]Species
	moves   map[string]Move
	loaded  bool
	loadErr error
}

// New returns a Dex that caches its data files in dir.
func New(dir string) *Dex {
	return &Dex{
		dir:    dir,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Loaded reports whether the catalogue has been read.
func (d *Dex) Loaded() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.loaded
}

// LoadErr returns the error from the last load attempt, if any.
func (d *Dex) LoadErr() error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.loadErr
}

// Ensure loads the catalogue, downloading it if the cache is empty. It is safe
// to call from a background goroutine; callers should not block the UI on it.
func (d *Dex) Ensure(ctx context.Context) error {
	d.mu.RLock()
	if d.loaded {
		err := d.loadErr
		d.mu.RUnlock()
		return err
	}
	d.mu.RUnlock()

	pokedex, err := d.dataFile(ctx, "pokedex.json")
	if err != nil {
		d.setErr(err)
		return err
	}
	movesRaw, err := d.dataFile(ctx, "moves.json")
	if err != nil {
		d.setErr(err)
		return err
	}

	species := make(map[string]Species, 1400)
	var rawSpecies map[string]struct {
		Name        string            `json:"name"`
		Num         int               `json:"num"`
		Types       []string          `json:"types"`
		BaseStats   map[string]int    `json:"baseStats"`
		Abilities   map[string]string `json:"abilities"`
		BaseSpecies string            `json:"baseSpecies"`
		Forme       string            `json:"forme"`
		Gen         int               `json:"gen"`
		Tier        string            `json:"tier"`
	}
	if err := json.Unmarshal(pokedex, &rawSpecies); err != nil {
		d.setErr(err)
		return fmt.Errorf("dex: pokedex: %w", err)
	}
	for k, v := range rawSpecies {
		species[k] = Species{
			Key:         k,
			Name:        v.Name,
			Num:         v.Num,
			Types:       v.Types,
			BaseStats:   v.BaseStats,
			Abilities:   v.Abilities,
			BaseSpecies: v.BaseSpecies,
			Forme:       v.Forme,
			Gen:         v.Gen,
			Tier:        v.Tier,
		}
	}

	moves := make(map[string]Move, 1000)
	var rawMoves map[string]struct {
		Name      string `json:"name"`
		Type      string `json:"type"`
		Category  string `json:"category"`
		PP        int    `json:"pp"`
		BasePower int    `json:"basePower"`
		Target    string `json:"target"`
		ShortDesc string `json:"shortDesc"`
	}
	if err := json.Unmarshal(movesRaw, &rawMoves); err != nil {
		d.setErr(err)
		return fmt.Errorf("dex: moves: %w", err)
	}
	for k, v := range rawMoves {
		moves[k] = Move{
			ID:        k,
			Name:      v.Name,
			Type:      v.Type,
			Category:  v.Category,
			PP:        v.PP,
			BasePower: v.BasePower,
			Target:    v.Target,
			ShortDesc: v.ShortDesc,
		}
	}

	d.mu.Lock()
	d.species, d.moves, d.loaded, d.loadErr = species, moves, true, nil
	d.mu.Unlock()
	return nil
}

func (d *Dex) setErr(err error) {
	d.mu.Lock()
	d.loadErr = err
	d.mu.Unlock()
}

// Species looks up a species by name or ID. It returns false when the
// catalogue has not loaded yet.
func (d *Dex) Species(name string) (Species, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if s, ok := d.species[ToID(name)]; ok {
		return s, true
	}
	return Species{}, false
}

// Move looks up a move by name or ID.
func (d *Dex) Move(name string) (Move, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if m, ok := d.moves[ToID(name)]; ok {
		return m, true
	}
	return Move{}, false
}

// dataFile returns the cached contents of a data file, downloading it if
// necessary.
func (d *Dex) dataFile(ctx context.Context, name string) ([]byte, error) {
	path := filepath.Join(d.dir, name)
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		return raw, nil
	}
	if err := os.MkdirAll(d.dir, 0o755); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BaseURL+"/"+name, nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("dex: fetch %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("dex: fetch %s: %s", name, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxDexBytes))
	if err != nil {
		return nil, err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("dex: %s is not valid JSON", name)
	}
	// Write via a temp file so a partial download never poisons the cache.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err == nil {
		_ = os.Rename(tmp, path)
	}
	return raw, nil
}

// ToID normalises a name to a Showdown identifier.
func ToID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		}
	}
	return b.String()
}
