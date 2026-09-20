package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestDefaultConfigIsValid(t *testing.T) {
	c := Default()
	if problems := c.Validate(); len(problems) != 0 {
		t.Fatalf("default config invalid: %v", problems)
	}
	if c.Sprites.Mode != "auto" {
		t.Errorf("default sprite mode = %q", c.Sprites.Mode)
	}
	if c.DefaultFormat != "gen9randombattle" {
		t.Errorf("default format = %q", c.DefaultFormat)
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	c := Default()
	c.Username = "coffee_enjoyer"
	c.Theme = "gameboy"
	c.Taglines = "absurd"
	c.Sprites.Animate = false
	c.Keybindings = map[string]string{"quit": "ctrl+q"}

	if err := c.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if strings.Contains(string(raw), "password") {
		t.Fatalf("config file must never contain a password field:\n%s", raw)
	}

	loaded := Default()
	if err := toml.Unmarshal(raw, &loaded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if loaded.Username != "coffee_enjoyer" || loaded.Theme != "gameboy" {
		t.Errorf("round trip lost values: %#v", loaded)
	}
	if loaded.Taglines != "absurd" || loaded.Sprites.Animate {
		t.Errorf("round trip lost nested values: %#v", loaded)
	}
	if loaded.Keybindings["quit"] != "ctrl+q" {
		t.Errorf("keybindings lost: %#v", loaded.Keybindings)
	}
}

func TestApplyDefaultsFillsBlanks(t *testing.T) {
	c := Config{}
	c.applyDefaults()
	if c.Theme == "" || c.DefaultFormat == "" || c.Taglines == "" || c.Notifications == "" {
		t.Errorf("defaults not applied: %#v", c)
	}
	if c.Sprites.Mode == "" || c.Sprites.Size == "" {
		t.Errorf("sprite defaults not applied: %#v", c.Sprites)
	}
}

func TestValidateRejectsUnknownValues(t *testing.T) {
	c := Default()
	c.Taglines = "shakespearean"
	c.Notifications = "carrier pigeon"
	c.Sprites.Mode = "hologram"
	problems := c.Validate()
	if len(problems) != 3 {
		t.Fatalf("want 3 problems, got %d: %v", len(problems), problems)
	}
}

func TestTaglineModes(t *testing.T) {
	c := Default()
	if got := c.Tagline(SlotYourTurn, false); got != "What will you do?" {
		t.Errorf("canon your_turn = %q", got)
	}
	if got := c.Tagline(SlotSplash, false); !strings.HasPrefix(got, "There's a time") {
		t.Errorf("canon splash = %q", got)
	}

	c.Taglines = "off"
	if got := c.Tagline(SlotYourTurn, false); got != "" {
		t.Errorf("off should return empty, got %q", got)
	}
	if got := c.EasterEgg(); got != "" {
		t.Errorf("off easter egg should be empty, got %q", got)
	}

	c.Taglines = "absurd"
	if got := c.Tagline(SlotIdle, false); got != "Snorlax used Rest." {
		t.Errorf("absurd idle = %q", got)
	}

	// Random must always return a member of the pool.
	c.Taglines = "canon"
	for i := 0; i < 50; i++ {
		got := c.Tagline(SlotEasterEgg, true)
		if got == "" {
			t.Fatal("random easter egg returned empty")
		}
	}
}

func TestEasterEggsIncludeTheRequestedLines(t *testing.T) {
	c := Default()
	want := []string{
		"I like shorts! They're comfy and easy to wear!",
		"My Rattata is in the top percentage of Rattata.",
		"It's not the time to use that!",
		"The ROAD is closed because of a landslide.",
	}
	have := map[string]bool{}
	for i := 0; i < 200; i++ {
		have[c.EasterEgg()] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("easter egg %q never appeared", w)
		}
	}
}
