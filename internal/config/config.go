// Package config loads and saves the human-readable TOML configuration.
//
// Passwords are never stored here. If the user opts into "remember", the
// password lives in the OS keyring (see internal/config/keyring.go).
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/unnipv/pokemon-slowdown/internal/storage"
)

// Config is the on-disk configuration.
type Config struct {
	// Username is the Pokémon Showdown name to log in as.
	Username string `toml:"username"`
	// Remember stores the password in the OS keyring so startup can log in
	// automatically. The password itself is never written to this file.
	Remember bool `toml:"remember"`
	// Theme selects a colour theme by name.
	Theme string `toml:"theme"`
	// DefaultFormat is the format queued by "slowdown rand" and the lobby.
	DefaultFormat string `toml:"default_format"`
	// Taglines is "canon", "absurd" or "off".
	Taglines string `toml:"taglines"`
	// TaglinesRandom rotates a random line instead of the first in each slot.
	TaglinesRandom bool `toml:"taglines_random"`
	// Notifications is "off", "bell", "osc" or "desktop".
	Notifications string `toml:"notifications"`
	// Debug writes sanitized protocol events to a log file.
	Debug bool `toml:"debug"`
	// NoColor disables all colour output. NO_COLOR is also honoured.
	NoColor bool `toml:"no_color"`
	// Sidecar starts in the narrow, coding-friendly layout.
	Sidecar bool `toml:"sidecar"`
	// Sprites configures the sprite renderer.
	Sprites Sprites `toml:"sprites"`
	// Keybindings overrides default key bindings.
	Keybindings map[string]string `toml:"keybindings"`
}

// Sprites configures terminal sprite rendering.
type Sprites struct {
	// Mode is "auto", "kitty", "iterm2", "sixel", "blocks" or "none".
	Mode string `toml:"mode"`
	// Animate enables animated GIF sprites where supported.
	Animate bool `toml:"animate"`
	// Size is "small", "medium" or "large".
	Size string `toml:"size"`
	// CacheDir overrides the sprite cache location.
	CacheDir string `toml:"cache_dir"`
}

// Default returns the built-in configuration.
func Default() Config {
	return Config{
		Theme:         "zen",
		DefaultFormat: "gen9randombattle",
		Taglines:      "canon",
		Notifications: "bell",
		Sprites: Sprites{
			Mode:    "auto",
			Animate: true,
			Size:    "medium",
		},
	}
}

// Load reads config.toml, returning defaults when it does not exist. The
// returned path is the file that was read (or would be written).
func Load() (Config, string, error) {
	path := storage.ConfigFile()
	cfg := Default()

	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, path, nil
	}
	if err != nil {
		return cfg, path, err
	}
	if err := toml.Unmarshal(raw, &cfg); err != nil {
		return cfg, path, fmt.Errorf("config %s: %w", path, err)
	}
	cfg.applyDefaults()
	return cfg, path, nil
}

// Save writes the configuration, creating the config directory if needed.
func (c Config) Save(path string) error {
	if path == "" {
		path = storage.ConfigFile()
	}
	if _, err := storage.EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := toml.NewEncoder(f)
	enc.Indent = "  "
	return enc.Encode(c)
}

// applyDefaults fills in values for keys a user left blank.
func (c *Config) applyDefaults() {
	d := Default()
	if c.Theme == "" {
		c.Theme = d.Theme
	}
	if c.DefaultFormat == "" {
		c.DefaultFormat = d.DefaultFormat
	}
	if c.Taglines == "" {
		c.Taglines = d.Taglines
	}
	if c.Notifications == "" {
		c.Notifications = d.Notifications
	}
	if c.Sprites.Mode == "" {
		c.Sprites.Mode = d.Sprites.Mode
	}
	if c.Sprites.Size == "" {
		c.Sprites.Size = d.Sprites.Size
	}
}

// Validate reports configuration values that are not recognised.
func (c Config) Validate() []string {
	var problems []string
	switch c.Taglines {
	case "canon", "absurd", "off":
	default:
		problems = append(problems, fmt.Sprintf("taglines: unknown value %q (want canon, absurd or off)", c.Taglines))
	}
	switch c.Notifications {
	case "off", "bell", "osc", "desktop":
	default:
		problems = append(problems, fmt.Sprintf("notifications: unknown value %q", c.Notifications))
	}
	switch c.Sprites.Mode {
	case "auto", "kitty", "iterm2", "sixel", "blocks", "none":
	default:
		problems = append(problems, fmt.Sprintf("sprites.mode: unknown value %q", c.Sprites.Mode))
	}
	switch c.Sprites.Size {
	case "small", "medium", "large":
	default:
		problems = append(problems, fmt.Sprintf("sprites.size: unknown value %q", c.Sprites.Size))
	}
	return problems
}
