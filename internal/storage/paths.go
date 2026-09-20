// Package storage resolves platform-appropriate config, cache and data
// directories following the XDG base directory specification on Unix.
package storage

import (
	"os"
	"path/filepath"
)

// AppName is the directory name used under the platform's config, cache and
// data roots.
const AppName = "pokemon-slowdown"

// ConfigDir returns the directory holding config.toml.
func ConfigDir() string {
	return filepath.Join(base("XDG_CONFIG_HOME", ".config"), AppName)
}

// CacheDir returns the directory holding downloaded sprites and dex data.
func CacheDir() string {
	return filepath.Join(base("XDG_CACHE_HOME", ".cache"), AppName)
}

// DataDir returns the directory holding teams and battle history.
func DataDir() string {
	return filepath.Join(base("XDG_DATA_HOME", ".local/share"), AppName)
}

// LogDir returns the directory holding debug logs.
func LogDir() string {
	return filepath.Join(base("XDG_STATE_HOME", ".local/state"), AppName, "logs")
}

// ConfigFile returns the path to config.toml.
func ConfigFile() string { return filepath.Join(ConfigDir(), "config.toml") }

// TeamsFile returns the path to the stored teams file.
func TeamsFile() string { return filepath.Join(DataDir(), "teams.json") }

// SpriteDir returns the sprite cache directory.
func SpriteDir() string { return filepath.Join(CacheDir(), "sprites") }

// DexDir returns the directory holding cached dex data.
func DexDir() string { return filepath.Join(CacheDir(), "dex") }

// EnsureDir creates dir (and parents) if needed and returns it.
func EnsureDir(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func base(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fallback
	}
	return filepath.Join(home, fallback)
}
