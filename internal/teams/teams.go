// Package teams models user-built teams and converts between the three
// representations the client needs: an in-memory team, Showdown's packed
// format for the server, and Showdown's human-readable export format for the
// terminal editor.
package teams

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Stats holds EV or IV values in canonical order.
type Stats struct {
	HP  int `json:"hp"`
	Atk int `json:"atk"`
	Def int `json:"def"`
	SpA int `json:"spa"`
	SpD int `json:"spd"`
	Spe int `json:"spe"`
}

// Pokemon is one team member.
type Pokemon struct {
	Name     string   `json:"name,omitempty"`
	Species  string   `json:"species"`
	Item     string   `json:"item,omitempty"`
	Ability  string   `json:"ability,omitempty"`
	TeraType string   `json:"tera_type,omitempty"`
	Nature   string   `json:"nature,omitempty"`
	Gender   string   `json:"gender,omitempty"`
	Shiny    bool     `json:"shiny,omitempty"`
	Level    int      `json:"level,omitempty"`
	EVs      Stats    `json:"evs,omitempty"`
	IVs      Stats    `json:"ivs,omitempty"`
	Moves    []string `json:"moves,omitempty"`
}

// Display returns the nickname if set, otherwise the species.
func (p Pokemon) Display() string {
	if p.Name != "" {
		return p.Name
	}
	return p.Species
}

// Team is a named collection of Pokémon.
type Team struct {
	Name    string    `json:"name"`
	Format  string    `json:"format,omitempty"`
	Pokemon []Pokemon `json:"pokemon"`
}

// Store persists teams as a single JSON document.
type Store struct {
	path  string
	Teams []Team
}

// Open loads the team store from path, returning an empty store if it does not
// exist.
func Open(path string) (*Store, error) {
	s := &Store{path: path}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(raw, s); err != nil {
		return nil, fmt.Errorf("teams %s: %w", path, err)
	}
	return s, nil
}

// Path returns the file backing the store.
func (s *Store) Path() string { return s.path }

// All returns a copy of the stored teams.
func (s *Store) All() []Team {
	out := make([]Team, len(s.Teams))
	copy(out, s.Teams)
	return out
}

// Get returns the team at index i.
func (s *Store) Get(i int) (Team, bool) {
	if i < 0 || i >= len(s.Teams) {
		return Team{}, false
	}
	return s.Teams[i], true
}

// Add appends a team and saves.
func (s *Store) Add(t Team) error {
	if t.Name == "" {
		t.Name = "New Team"
	}
	s.Teams = append(s.Teams, t)
	return s.Save()
}

// Put replaces the team at index i and saves.
func (s *Store) Put(i int, t Team) error {
	if i < 0 || i >= len(s.Teams) {
		return fmt.Errorf("teams: no team at index %d", i)
	}
	s.Teams[i] = t
	return s.Save()
}

// Delete removes the team at index i and saves.
func (s *Store) Delete(i int) error {
	if i < 0 || i >= len(s.Teams) {
		return fmt.Errorf("teams: no team at index %d", i)
	}
	s.Teams = append(s.Teams[:i], s.Teams[i+1:]...)
	return s.Save()
}

// Duplicate copies the team at index i with a new name.
func (s *Store) Duplicate(i int) error {
	t, ok := s.Get(i)
	if !ok {
		return fmt.Errorf("teams: no team at index %d", i)
	}
	t.Name = uniqueName(t.Name, s.Teams)
	s.Teams = append(s.Teams, t)
	return s.Save()
}

// Save writes the store atomically.
func (s *Store) Save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Find locates a team by name, case-insensitively.
func (s *Store) Find(name string) (int, bool) {
	for i, t := range s.Teams {
		if strings.EqualFold(t.Name, name) {
			return i, true
		}
	}
	return -1, false
}

func uniqueName(base string, existing []Team) string {
	taken := map[string]bool{}
	for _, t := range existing {
		taken[strings.ToLower(t.Name)] = true
	}
	candidate := base + " (copy)"
	for n := 2; taken[strings.ToLower(candidate)]; n++ {
		candidate = fmt.Sprintf("%s (copy %d)", base, n)
	}
	return candidate
}
