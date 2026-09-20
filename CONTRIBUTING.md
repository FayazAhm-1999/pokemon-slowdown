# Contributing

Thanks for wanting to help. This is a small, focused project: a terminal client
for Pokémon Showdown that stays out of your way while you code.

## Prerequisites

- **Go 1.27 or newer** (`go version`)
- A terminal with truecolour support for development. Ghostty, Kitty, WezTerm or
  iTerm2 give you real pixel sprites; anything else exercises the half-block
  fallback, which is worth testing too.
- Optional: `staticcheck` for linting.

```sh
brew install go
go install honnef.co/go/tools/cmd/staticcheck@latest
```

## Getting started

```sh
git clone https://github.com/unnipv/pokemon-slowdown
cd pokemon-slowdown

make build          # -> ./slowdown
make run            # build and open the lobby
make test           # unit and protocol tests, no network
```

Or without make:

```sh
go build -o slowdown ./cmd/slowdown
go run ./cmd/slowdown
```

You can play immediately as a guest. `slowdown rand` queues a Gen 9 Random
Battle; `slowdown doctor` prints terminal diagnostics.

## The checks that must pass

```sh
make check          # gofmt, go vet, staticcheck, tests
```

Individually:

```sh
gofmt -l ./cmd ./internal   # must print nothing
go vet ./...
staticcheck ./...
go test ./...
go test -race ./...
```

CI runs all of these on Linux, macOS and Windows, plus a GoReleaser snapshot
build, so a PR that breaks any of them will be caught.

## Tests that touch the real world

Two suites are opt-in because they hit live services:

```sh
SLOWDOWN_LIVE=1 go test ./internal/showdown/ -run TestLive -v   # server, login, formats
SLOWDOWN_LIVE=1 go test ./internal/sprites/  -run TestLive -v   # sprite downloads and rendering
SLOWDOWN_LIVE=1 go test ./internal/tui/ -run TestLive -v        # sprites reaching the terminal
```

The sprite ones are the interesting ones. They download real sprites, render
them, parse the ANSI back into pixels and compare against the source image, so a
renderer that silently produces a blank box fails.

There is also a layout preview that prints a real mid-battle screen at several
widths, which is the fastest way to eyeball a layout change:

```sh
SLOWDOWN_LIVE=1 go test ./internal/tui/ -run TestLayoutPreview -v
```

## Project layout

```
cmd/slowdown/        CLI entry point and subcommands
internal/showdown/   wire protocol: framing, typed events, parser, auth, client
internal/battle/     battle state, request decoding, choices, reducer
internal/sprites/    renderer backends, capability detection, download cache
internal/teams/      team model, packed format, import/export, storage
internal/tui/        screens, overlays, themes, responsive layout
internal/dex/        cached species and move reference data
internal/config/     TOML config, keyring, taglines
internal/storage/    XDG paths
internal/notify/     bell, OSC and desktop notifications
testdata/battles/    recorded battle streams replayed by the tests
```

The dependency direction is one way: `tui` and `app` depend on everything,
everything else depends on nothing above it. In particular `showdown` and
`battle` must never import `tui` or `lipgloss`.

## Adding a protocol event

The protocol is a moving target, so this is the most common kind of change.

1. Add a typed event to `internal/showdown/events.go`.
2. Parse it in `internal/showdown/parser.go` (`parseBattle` for battle
   messages). Anything you do not model is preserved as `BattleEffect` or
   `BattleUnknown`, so nothing is ever silently dropped.
3. Handle it in `internal/battle/reducer.go`, and log a human-readable line
   there — the battle log is the user's only window into what happened.
4. Route it in `internal/tui/app.go` if the UI needs to see it.
5. Add a fixture line to `testdata/battles/gen9-singles.txt` and an assertion
   in `internal/battle/reducer_test.go`.

Two things to watch out for, both of which have bitten this project:

- **Every outbound message needs a `|`.** The server drops any client message
  without a room separator, so a global command is `|/search gen9randombattle`,
  not `/search gen9randombattle`. `TestOutboundMessagesAlwaysContainPipe` guards
  this.
- **Replies to roomless commands arrive as PMs**, not room messages. If you add
  a command, check how its failures surface.

See `docs/IMPLEMENTATION_NOTES.md` for the full list of protocol discoveries and
renderer compromises.

## Rendering changes

Two rules worth knowing before touching sprites:

- Bubble Tea **discards graphics escapes** in view content. Pixel sprites are
  drawn out of band via a sentinel rune substituted in the output writer. Do not
  try to put an escape in a `View` string; it will silently vanish.
- The half-block renderer is the universal fallback and must always work. If you
  change it, run the live sprite round-trip test, which asserts that rendered
  pixels match the source image.

## Style

- Idiomatic Go. `gofmt` decides formatting; do not argue with it.
- Prefer the standard library. A dependency has to earn its place.
- Comments should explain *why*, especially where protocol behaviour is
  non-obvious or surprising. Several comments in this codebase exist purely to
  stop someone re-introducing a bug.
- No global mutable state. The protocol, battle state and rendering layers are
  all testable without a terminal.
- Treat everything from the server as untrusted. Run it through
  `tui.Sanitize` before rendering, and never interpolate it into a shell
  command.

## Reporting bugs

Please include `slowdown doctor` output. It reports your terminal, `TERM`,
multiplexer detection, detected graphics protocols, the selected sprite backend
and every config/cache/data path, which is usually enough to reproduce.

For rendering problems, `slowdown doctor --sprites` shows which backends your
terminal actually honours.

## Licence

By contributing you agree your work is licensed under the MIT licence, and that
you have the right to submit it. Do not paste code from Pokémon Showdown's
AGPL-licensed web client — reimplement from the documented protocol instead.
