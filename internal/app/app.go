// Package app wires the client, dex, sprites and UI together and owns process
// lifecycle: configuration, credentials, logging, terminal cleanup and the
// doctor diagnostic.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/unnipv/pokemon-slowdown/internal/config"
	"github.com/unnipv/pokemon-slowdown/internal/dex"
	"github.com/unnipv/pokemon-slowdown/internal/notify"
	"github.com/unnipv/pokemon-slowdown/internal/showdown"
	"github.com/unnipv/pokemon-slowdown/internal/sprites"
	"github.com/unnipv/pokemon-slowdown/internal/storage"
	"github.com/unnipv/pokemon-slowdown/internal/teams"
	"github.com/unnipv/pokemon-slowdown/internal/tui"
)

// Options are the resolved command-line options.
type Options struct {
	// Format, when set, starts a ladder search as soon as the client is ready.
	Format string
	// Spectate joins a battle room as a spectator.
	Spectate string
	// Challenge is a username to challenge once connected.
	Challenge string
	// Debug writes sanitized protocol events to a log file.
	Debug bool
	// SpritesMode overrides sprites.mode.
	SpritesMode string
	// Theme overrides the configured theme.
	Theme string
	// NoSprites forces sprites off.
	NoSprites bool
	// OpenTeams starts on the team management screen.
	OpenTeams bool
	// Version is reported by --version.
	Version string
}

// Run starts the application and blocks until it exits.
func Run(opts Options) error {
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	applyOverrides(&cfg, opts)

	if problems := cfg.Validate(); len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "pokemon slowdown: configuration problems:")
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "  - "+p)
		}
		fmt.Fprintln(os.Stderr, "  continuing with defaults for those keys")
	}

	// Logging must never corrupt the TUI, so it goes to a file.
	logger, logPath, closeLog := setupLogging(cfg.Debug)
	defer closeLog()
	if cfg.Debug && logPath != "" {
		fmt.Fprintln(os.Stderr, "debug log:", logPath)
	}

	// Sprite cache.
	spriteDir := cfg.Sprites.CacheDir
	if spriteDir == "" {
		spriteDir, err = storage.EnsureDir(storage.SpriteDir())
		if err != nil {
			spriteDir = filepath.Join(os.TempDir(), "pokemon-slowdown-sprites")
		}
	}
	dexDir, err := storage.EnsureDir(storage.DexDir())
	if err != nil {
		dexDir = os.TempDir()
	}

	// Credentials come from the config plus, optionally, the OS keyring.
	creds := showdown.Credentials{Username: cfg.Username}
	if cfg.Remember && cfg.Username != "" {
		if pw, err := config.LoadPassword(cfg.Username); err == nil {
			creds.Password = pw
		}
	}

	client := showdown.NewClient(showdown.DefaultURL)
	client.Debug = cfg.Debug
	if creds.Username != "" {
		client.SetCredentials(creds)
	}

	renderer, err := sprites.NewRenderer(sprites.ParseMode(cfg.Sprites.Mode), nil)
	if err != nil {
		renderer, _ = sprites.NewRenderer(sprites.ModeBlocks, nil)
	}

	teamStore, err := teams.Open(storage.TeamsFile())
	if err != nil {
		teamStore = nil
	}

	deps := tui.Deps{
		Client:   client,
		Dex:      dex.New(dexDir),
		Sprites:  sprites.NewManager(spriteDir),
		Renderer: renderer,
		Notifier: notify.New(notify.Mode(cfg.Notifications)),
		Teams:    teamStore,
		Debug:    cfg.Debug,
		Logf: func(format string, args ...any) {
			if logger != nil {
				logger.Printf(format, args...)
			}
		},
	}

	model := tui.New(cfg, deps)
	model.SetAutoQueue(opts.Format)
	model.SetPendingSpectate(opts.Spectate)
	model.SetPendingChallenge(opts.Challenge)
	if opts.OpenTeams {
		model.SetStartScreenTeams()
	}

	client.Start()

	// With a pixel backend, sprites are drawn out of band: Bubble Tea strips
	// graphics escapes from view content, so the layer rewrites sentinel runes
	// in the output stream instead.
	var progOpts []tea.ProgramOption
	if sprites.SupportsPayload(renderer) {
		progOpts = append(progOpts, tea.WithOutput(model.SpriteWriter(os.Stdout)))
	}
	program := tea.NewProgram(model, progOpts...)

	// Restore the terminal on every exit path, including panics.
	cleanup := func() {
		sprites.Cleanup(os.Stdout, renderer.Capabilities())
		client.Close()
		_ = renderer.Close()
	}
	defer func() {
		if r := recover(); r != nil {
			cleanup()
			fmt.Fprintln(os.Stderr, "pokemon slowdown: unexpected error:", r)
			if cfg.Debug {
				fmt.Fprintln(os.Stderr, string(debug.Stack()))
			}
			return
		}
		cleanup()
	}()

	_, err = program.Run()
	if errors.Is(err, tea.ErrProgramKilled) {
		// A user interrupt (Ctrl+C) or signal is a clean exit, not a failure.
		return nil
	}
	return err
}

func applyOverrides(cfg *config.Config, opts Options) {
	if opts.Debug {
		cfg.Debug = true
	}
	if opts.Theme != "" {
		cfg.Theme = opts.Theme
	}
	if opts.NoSprites {
		cfg.Sprites.Mode = "none"
	} else if opts.SpritesMode != "" {
		cfg.Sprites.Mode = opts.SpritesMode
	}
}

// setupLogging opens the debug log file. It returns the logger, the path and a
// close function. Logs never contain credentials: only sanitized protocol
// events are ever written.
func setupLogging(debug bool) (*log.Logger, string, func()) {
	if !debug {
		return nil, "", func() {}
	}
	dir, err := storage.EnsureDir(storage.LogDir())
	if err != nil {
		return nil, "", func() {}
	}
	path := filepath.Join(dir, "slowdown.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, "", func() {}
	}
	logger := log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	return logger, path, func() { _ = f.Close() }
}

// Doctor writes a diagnostic report about the terminal and configuration.
func Doctor(w io.Writer, showSprites bool) error {
	cfg, cfgPath, err := config.Load()
	if err != nil {
		return err
	}
	caps := sprites.Detect()
	mode := sprites.ParseMode(cfg.Sprites.Mode)
	selected, err := sprites.NewRenderer(mode, nil)
	if err != nil {
		return err
	}

	fmt.Fprintln(w, "pokemon slowdown doctor")
	fmt.Fprintln(w, "──────────────────────────────────────────────")
	fmt.Fprintf(w, "terminal        %s\n", envOr(os.Getenv("TERM_PROGRAM"), "(unknown)"))
	fmt.Fprintf(w, "TERM            %s\n", envOr(os.Getenv("TERM"), "(unset)"))
	fmt.Fprintf(w, "COLORTERM       %s\n", envOr(os.Getenv("COLORTERM"), "(unset)"))
	fmt.Fprintf(w, "multiplexer     %s\n", tmuxLabel(caps.Tmux))
	fmt.Fprintf(w, "truecolour      %s\n", truecolourLabel())
	fmt.Fprintf(w, "graphics        detected %s\n", caps.Detected)
	fmt.Fprintf(w, "sprite backend  %s (configured %q)\n", selected.Capabilities().Protocol, cfg.Sprites.Mode)
	if caps.CellWidth > 0 && caps.CellHeight > 0 {
		fmt.Fprintf(w, "cell size       %dx%d px\n", caps.CellWidth, caps.CellHeight)
	} else {
		fmt.Fprintf(w, "cell size       unknown\n")
	}
	fmt.Fprintf(w, "notifications   %s\n", cfg.Notifications)
	fmt.Fprintf(w, "config          %s\n", cfgPath)
	fmt.Fprintf(w, "config dir      %s\n", storage.ConfigDir())
	fmt.Fprintf(w, "cache dir       %s\n", storage.CacheDir())
	fmt.Fprintf(w, "data dir        %s\n", storage.DataDir())
	fmt.Fprintf(w, "log dir         %s\n", storage.LogDir())
	fmt.Fprintf(w, "theme           %s\n", cfg.Theme)
	fmt.Fprintf(w, "taglines        %s\n", cfg.Taglines)

	if showSprites {
		fmt.Fprintln(w, "\nsprite backend probe")
		fmt.Fprintln(w, "──────────────────────────────────────────────")
		if err := probeSprites(w, cfg); err != nil {
			fmt.Fprintln(w, "probe failed:", err)
		}
	}
	return nil
}

// probeSprites draws one test sprite through each backend so users can see
// which one their terminal actually renders.
func probeSprites(w io.Writer, cfg config.Config) error {
	mgr := sprites.NewManager(filepath.Join(os.TempDir(), "slowdown-doctor-sprites"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sprite, err := mgr.Load(ctx, sprites.Ref{ID: "pikachu"})
	if err != nil {
		return err
	}
	for _, mode := range []sprites.Mode{sprites.ModeBlocks, sprites.ModeKitty, sprites.ModeITerm2, sprites.ModeSixel} {
		r, err := sprites.NewRenderer(mode, nil)
		if err != nil {
			continue
		}
		out, err := r.Render(sprite.Static, 10, 5)
		if err != nil {
			fmt.Fprintf(w, "%s: %v\n", mode, err)
			continue
		}
		fmt.Fprintf(w, "%s:\n%s\n\n", mode, out)
	}
	return nil
}

func envOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func tmuxLabel(tmux bool) string {
	if tmux {
		return "yes (graphics fall back to half-blocks)"
	}
	return "no"
}

func truecolourLabel() string {
	ct := os.Getenv("COLORTERM")
	if ct == "truecolor" || ct == "24bit" {
		return "yes (COLORTERM)"
	}
	if os.Getenv("TERM_PROGRAM") != "" {
		return "likely (modern terminal)"
	}
	return "unknown"
}

// NotificationModeLabel exposes the notify mode for the doctor output.
func NotificationModeLabel(m notify.Mode) string { return string(m) }
