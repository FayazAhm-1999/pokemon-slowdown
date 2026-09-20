// Command slowdown is a terminal-native Pokémon Showdown client.
//
// Usage:
//
//	slowdown                     open the lobby
//	slowdown rand                queue a random battle immediately
//	slowdown play <format>       queue a specific format
//	slowdown challenge <user>    challenge a user
//	slowdown spectate <battle>   watch a battle
//	slowdown teams               open team management
//	slowdown doctor [--sprites]  print terminal diagnostics
//	slowdown --version
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/unnipv/pokemon-slowdown/internal/app"
	"runtime/debug"
)

// version is set at build time by GoReleaser via -ldflags.
var version = "dev"

// versionString reports the release version. Binaries built by GoReleaser carry
// it in the version variable; binaries installed with `go install` do not get
// ldflags, but the module version is recorded in the build info, so read that
// instead of reporting "dev".
func versionString() string {
	if version != "dev" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := bi.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return version
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "slowdown:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("slowdown", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		showVersion = fs.Bool("version", false, "print the version and exit")
		debug       = fs.Bool("debug", false, "write sanitized protocol events to a log file")
		noSprites   = fs.Bool("no-sprites", false, "disable sprites entirely")
		spritesMode = fs.String("sprites", "", "sprite backend: auto|kitty|iterm2|sixel|blocks|none")
		theme       = fs.String("theme", "", "colour theme: "+strings.Join(themeNames(), ", "))
	)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: slowdown [command] [flags]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "commands:")
		fmt.Fprintln(os.Stderr, "  (none)              open the lobby")
		fmt.Fprintln(os.Stderr, "  rand                queue a random battle")
		fmt.Fprintln(os.Stderr, "  play <format>       queue a format, e.g. play gen9ou")
		fmt.Fprintln(os.Stderr, "  challenge <user>    challenge a user to your default format")
		fmt.Fprintln(os.Stderr, "  spectate <battle>   watch a battle id or replay URL")
		fmt.Fprintln(os.Stderr, "  teams               open team management")
		fmt.Fprintln(os.Stderr, "  doctor [--sprites]  print terminal diagnostics")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "flags:")
		fs.PrintDefaults()
	}

	// Split off the subcommand before flag parsing so "slowdown play gen9ou"
	// works without the flags parser eating the format.
	cmd := ""
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		rest = args[1:]
	}

	// doctor has its own flag set: --sprites is a boolean probe there, but a
	// string backend selector everywhere else.
	if cmd == "doctor" {
		dfs := flag.NewFlagSet("slowdown doctor", flag.ContinueOnError)
		probe := dfs.Bool("sprites", false, "render a test sprite through every backend")
		if err := dfs.Parse(rest); err != nil {
			return err
		}
		return app.Doctor(os.Stdout, *probe)
	}

	if err := fs.Parse(rest); err != nil {
		return err
	}

	if *showVersion {
		fmt.Println("pokemon slowdown", versionString())
		return nil
	}

	opts := app.Options{
		Debug:       *debug,
		NoSprites:   *noSprites,
		SpritesMode: *spritesMode,
		Theme:       *theme,
		Version:     version,
	}

	switch cmd {
	case "":
		// plain launch
	case "rand":
		opts.Format = "gen9randombattle"
	case "play":
		if fs.NArg() < 1 {
			return fmt.Errorf("play needs a format, e.g. slowdown play gen9ou")
		}
		opts.Format = fs.Arg(0)
	case "challenge":
		if fs.NArg() < 1 {
			return fmt.Errorf("challenge needs a username")
		}
		opts.Challenge = fs.Arg(0)
	case "spectate":
		if fs.NArg() < 1 {
			return fmt.Errorf("spectate needs a battle id or replay URL")
		}
		opts.Spectate = fs.Arg(0)
	case "teams":
		opts.OpenTeams = true
	case "version":
		fmt.Println("pokemon slowdown", versionString())
		return nil
	default:
		return fmt.Errorf("unknown command %q (try slowdown --help)", cmd)
	}

	return app.Run(opts)
}

func themeNames() []string {
	return []string{"zen", "dark", "light", "gameboy", "mono", "contrast"}
}
