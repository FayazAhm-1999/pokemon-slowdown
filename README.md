# pokemon slowdown

**A calm, keyboard-first terminal client for Pokémon Showdown.**

> There's a time and place for everything

Same battles. A calmer you. `slowdown` is a native terminal client built for
people who keep a million tabs open and would rather not add another one. It
lives in a split pane beside your editor, plays the full game, and gets out of
the way.

- **Keyboard-first.** Most singles turns are one keystroke. No mouse, ever.
- **Genuinely responsive.** Cinematic, standard, sidecar and compact layouts,
  each designed for its width rather than squeezed into it.
- **Terminal-native.** Real sprites in the terminal, with a half-block
  fallback that works literally everywhere.
- **Protocol-correct.** Singles, doubles, team preview, forced switches, Tera,
  Mega, Z-Moves and Dynamax, all driven by what the server actually sends.
- **Safe by construction.** Server text is sanitized before it is rendered, and
  the terminal is restored on every exit path.

```
┌ showdown · Gen 9 Random Battle ───────────────── Turn 11 ─┐
│  opponent · coffee_enjoyer                                │
│  Dragapult                                   71%  PAR     │
│        ··············                                     │
│        ··············                                     │
│  you · sleepy_panda                                       │
│  Kingambit                                   86%          │
│        ··············                                     │
├───────────────────────────────────────────────────────────┤
│  1 Kowtow Cleave   DARK    16/16                          │
│  2 Sucker Punch    DARK     7/8                           │
│  3 Iron Head       STEEL   23/24                          │
│  4 Swords Dance    STATUS  31/32                          │
│  [t] Tera   [s] switch                                    │
└───────────────────────────────────────────────────────────┘
```

## Install

### Homebrew (macOS and Linux)

```sh
brew install unnipv/tap/pokemon-slowdown
```

### Go

```sh
go install github.com/unnipv/pokemon-slowdown/cmd/slowdown@latest
```

### Release binaries

Download the archive for your platform from the
[releases page](https://github.com/unnipv/pokemon-slowdown/releases) and put
`slowdown` on your `PATH`. Checksums are published alongside each release.

## First run

```sh
slowdown
```

You start as a guest, which is enough to play Random Battles immediately. To
log in to a registered account, set `username` in your config and either type
your password when prompted or opt into the OS keyring:

```toml
# ~/.config/pokemon-slowdown/config.toml
username = "coffee_enjoyer"
remember = true   # stores the password in your OS keychain, never on disk
```

Passwords are **never** written to the config file. With `remember = true` the
password goes into the platform keychain (macOS Keychain, GNOME Keyring, Windows
Credential Manager). If no keychain is available, `slowdown` simply asks again
instead of storing anything.

## Commands

```sh
slowdown                     # open the lobby
slowdown rand                # queue a Gen 9 Random Battle immediately
slowdown play gen9ou         # queue a specific format
slowdown challenge <user>    # challenge someone
slowdown spectate <battle>   # watch a battle id or replay URL
slowdown teams               # team management
slowdown doctor              # print terminal diagnostics
slowdown doctor --sprites    # render a test sprite through every backend
slowdown --version
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--debug` | write sanitized protocol events to a log file |
| `--no-sprites` | disable sprites entirely |
| `--sprites <mode>` | `auto`, `kitty`, `iterm2`, `sixel`, `blocks`, `none` |
| `--theme <name>` | `zen`, `dark`, `light`, `gameboy`, `mono`, `contrast` |

## Controls

| Key | Action |
| --- | --- |
| `1`–`4` | choose a move |
| `s` | switch (then `1`–`6`, or arrow keys and enter) |
| `t` | use the current mechanic — Tera, Mega, Z-Move or Dynamax |
| `i` | inspect the selected Pokémon |
| `l` | battle log |
| `c` | battle chat |
| `tab` | cycle between battles |
| `:` | command palette |
| `?` | contextual help |
| `esc` | close an overlay, or return to the lobby |
| `q` | quit — asks first during a live battle |
| `ctrl+c` | quit immediately |

In doubles, a move that needs a target opens a target picker with numeric
shortcuts. Spread moves and self-targeting moves resolve without one.

`q` never forfeits by accident: a live battle always asks for confirmation
first. Forfeiting is an explicit command in the palette.

## Layouts

The layout is chosen from the terminal width, and each mode is designed rather
than compressed:

| Width | Mode | Behaviour |
| --- | --- | --- |
| ≥ 100 | cinematic | full battlefield, large sprites |
| 70–99 | standard | normal experience, medium sprites |
| 46–69 | sidecar | built for coding alongside; small sprites, moves always visible |
| < 46 | compact | gameplay only, no sprites, always usable |

Below 32×12 the app shows a graceful "terminal too small" state instead of
corrupting the screen.

## Sprites

Sprites are downloaded on demand from the public Pokémon Showdown sprite server
and cached locally. Loading never blocks input: the sprite box is reserved
immediately and a quiet placeholder is shown until the image arrives.

Backends, in order of fidelity:

| Backend | Notes |
| --- | --- |
| Kitty graphics | Ghostty, Kitty, WezTerm |
| iTerm2 inline images | iTerm2 |
| Sixel | foot, mlterm, contour, WezTerm |
| **Half-block** | **the default; works in every truecolour terminal** |

The half-block renderer is not a consolation prize. It preserves transparency
against your terminal background, keeps the aspect ratio, and is pure text, so
it is immune to the graphics glitches that pixel protocols can suffer inside a
full-screen TUI.

`auto` deliberately selects **half-block**. Pixel backends are opt-in via
`sprites.mode` because terminal graphics inside a continuously re-rendered TUI
are fragile; `slowdown doctor --sprites` lets you preview each one before you
commit. Inside tmux, `auto` always uses half-blocks, because graphics
passthrough is frequently unreliable there.

Animated GIF sprites are supported (`sprites.animate = true`). If a terminal
cannot animate cleanly, a high-quality static frame is used instead.

## Configuration

`~/.config/pokemon-slowdown/config.toml` (or `$XDG_CONFIG_HOME`). Every key is
optional.

```toml
username = ""
remember = false
theme = "zen"                     # zen | dark | light | gameboy | mono | contrast
default_format = "gen9randombattle"
taglines = "canon"                # canon | absurd | off
taglines_random = false           # rotate lines instead of using the first
notifications = "bell"            # off | bell | osc | desktop
debug = false
no_color = false
sidecar = false

[sprites]
mode = "auto"                     # auto | kitty | iterm2 | sixel | blocks | none
animate = true
size = "medium"                   # small | medium | large
# cache_dir = "/custom/path"

[keybindings]
# override any binding
```

`NO_COLOR` is honoured.

## Notifications

When a background battle becomes your turn, `slowdown` can ring the terminal
bell, emit an OSC 9 / OSC 777 desktop notification, or shell out to a platform
helper. It does not notify for the battle you are already looking at.

## Teams

`slowdown teams` lists your stored teams. Import a team by pasting Showdown
export text into the palette's *Import team* command, and export by reading the
stored team back out. The packed format used for the server is handled
internally, so you never have to think about it.

## Troubleshooting

Start with `slowdown doctor`. It reports your terminal, `TERM`, multiplexer
detection, truecolour support, the graphics protocols it detected, the selected
sprite backend, and every config/cache/data path.

**Sprites look like coloured blocks.** That is the half-block renderer working
as intended. Try `--sprites kitty` (Ghostty/Kitty/WezTerm) or `--sprites iterm2`
and compare with `slowdown doctor --sprites`.

**Sprites look wrong inside tmux.** That is expected; `auto` uses half-blocks
under tmux on purpose.

**The screen is garbled after quitting.** Please open an issue with
`slowdown doctor` output. Terminal restoration is a hard requirement, so this is
a bug worth reporting.

**"Terminal too small".** The window is below 32×12. Resize to continue.

## Privacy

- Passwords live in your OS keychain, never in the config file.
- Debug logs contain sanitized protocol events only — no passwords, no
  assertions, no cookies, no auth tokens.
- Server text (chat, usernames, room titles) is stripped of terminal control
  sequences before rendering, so another user cannot inject escape codes into
  your terminal.
- No telemetry. No analytics. No accounts beyond your Pokémon Showdown account.

## Contributing

```sh
go test ./...          # unit, protocol and reducer tests
go test -race ./...
go vet ./...
staticcheck ./...
gofmt -l ./cmd ./internal
```

Protocol fixtures live in `testdata/battles/` and are replayed through the
parser and reducer, so a change to protocol handling is verifiable without a
network connection.

`docs/IMPLEMENTATION_NOTES.md` records protocol discoveries, renderer
compromises and the manual terminal test matrix.

## Licence

MIT. See [LICENSE](LICENSE) and [THIRD_PARTY.md](THIRD_PARTY.md) for asset and
trademark attribution.
