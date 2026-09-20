You are the principal engineer responsible for building and shipping a complete open-source terminal-native Pokémon Showdown client.

This is NOT a prototype, proof of concept, architecture exercise, or MVP.

Build the actual product to a state where it can be published on GitHub, installed by normal users, connected to Pokémon Showdown, and used as a serious daily client.

Do not stop after scaffolding, protocol parsing, a demo battle, or a partially functional UI. Keep implementing, testing, fixing and polishing until the acceptance criteria near the end of this prompt are met.

# PRODUCT IDEA

Build a beautiful, extremely fast, keyboard-first Pokémon Showdown TUI intended to live in a terminal tab or narrow split while someone is coding.

The user should be able to:

- open a terminal
- run `showdown`
- log into Pokémon Showdown
- queue a ladder game or challenge someone
- play the entire battle comfortably without touching a mouse
- see attractive Pokémon sprites inside the terminal
- leave a slow battle running beside Neovim/Codex/etc.
- get notified when it is their turn
- resize the pane aggressively without breaking the UI
- use a full-screen cinematic layout or tiny sidecar layout
- spectate games
- chat if desired
- manage/select teams
- reconnect after temporary connection loss
- play singles and doubles correctly
- use modern battle mechanics such as Tera, Dynamax, Mega Evolution and Z-Moves whenever applicable

The guiding principle is:

"Do not recreate the Pokémon Showdown website. Reimagine Pokémon Showdown as if it had originally been designed as a terminal application."

The web app is a protocol/data reference, NOT a UI specification.

# TECHNOLOGY

Use Go.

Use current stable releases when implementing, not stale snippets.

Preferred stack:

- Go
- Bubble Tea v2
- Lip Gloss v2
- Bubbles v2 where useful
- go-termimg for terminal raster graphics where it proves reliable
- standard `image`, `image/gif`, etc. for image manipulation
- a well-maintained Go WebSocket library
- net/http for HTTP requests
- encoding/json
- keyring integration where practical for stored credentials/session material
- XDG/platform-appropriate config and cache directories
- GoReleaser for releases
- GitHub Actions for CI/release checks

Do not introduce Node, Python, Rust, Electron, a browser runtime or a webview.

The distributed application should be a native Go executable.

# REQUIRED RESEARCH BEFORE IMPLEMENTATION

Read the current upstream documentation and code before making assumptions.

At minimum inspect:

1. Pokémon Showdown:
   - PROTOCOL.md
   - sim/SIM-PROTOCOL.md
   - sim/TEAMS.md where needed
   - current authentication/login flow
   - current battle request formats
   - format list/search/challenge messages
   - reconnect behaviour
   - the official client ONLY as behavioural reference where documentation is insufficient

2. Current Bubble Tea v2 / Lip Gloss v2 APIs.

3. Current go-termimg APIs and known limitations.

4. Current Smogon sprite paths/assets.

5. Existing terminal Showdown clients for lessons, but do not copy their architecture blindly:
   - NixPSclient
   - old pokemon-showdown-terminal-client
   - starmie-cli
   - any other relevant maintained project you find

Prefer official Showdown protocol documentation over reverse engineering.

Do not copy AGPL client code into this project unless there is an explicit and intentional licensing reason to do so. Reimplement the protocol independently from its documented behaviour.

# REPOSITORY / BINARY

Repository working name:

`showdown-tui`

Binary:

`showdown`

The name can be changed later. Do not let naming block implementation.

Target:

- macOS arm64
- macOS amd64
- Linux amd64
- Linux arm64
- Windows amd64 where terminal functionality permits

The application must run well in at least:

- Ghostty
- Kitty
- iTerm2
- WezTerm
- common ANSI truecolour terminals using fallback rendering

tmux should be supported sensibly.

# CORE ARCHITECTURE

Keep protocol/domain/UI concerns strictly separate.

Use roughly this structure, adjusting when good Go design suggests something better:

showdown-tui/
  cmd/showdown/
  internal/
    app/
    showdown/
      connection.go
      auth.go
      protocol.go
      parser.go
      commands.go
      reconnect.go
      formats.go
    battle/
      state.go
      events.go
      request.go
      choices.go
      reducer.go
      targeting.go
      mechanics.go
    sprites/
      manager.go
      resolver.go
      cache.go
      renderer.go
      kitty.go
      iterm.go
      sixel.go
      halfblock.go
      animation.go
      detect.go
    teams/
      model.go
      storage.go
      packed.go
      import_export.go
    tui/
      app.go
      routes.go
      keymap.go
      theme.go
      responsive.go
      screens/
      components/
      overlays/
    notify/
    config/
    storage/
  testdata/
    protocol/
    battles/
    requests/
  docs/
  .github/
  .goreleaser.yaml

Do not make this a giant `main.go`.

# EVENT MODEL

The network package should parse raw Showdown messages into typed Go events.

Examples conceptually:

JoinedRoom
LeftRoom
UpdateUser
FormatsUpdated
SearchUpdated
ChallengesUpdated
ChatMessage
BattleStarted
BattleRequest
BattleMove
BattleSwitch
BattleDamage
BattleHeal
BattleStatus
BattleWeather
BattleFieldEffect
BattleTurn
BattleWin
BattleTie
BattleTimer
BattleError
etc.

The UI must NEVER need to manually parse strings like:

`|move|p1a: Dragapult|Draco Meteor|p2a: Garchomp`

The battle reducer consumes typed events and updates deterministic BattleState.

Preserve unknown/unimplemented protocol events rather than crashing. Log them in debug mode so protocol drift can be diagnosed.

# TESTING THE PROTOCOL

Create realistic protocol fixtures from documented/sample/replay-compatible battle streams.

Tests should be able to feed complete battle streams into the reducer and assert state after individual turns.

Include fixtures for:

- singles
- doubles
- team preview
- forced switch
- simultaneous fainting
- status
- hazards
- weather
- terrain
- boosts
- substitutions
- transform/illusion-like identity complications
- tera
- mega
- Z-moves
- dynamax
- choice errors
- undo where supported
- timers
- forfeits
- spectators
- reconnect/rejoin

Protocol/state correctness matters more than making every animation fancy.

# AUTHENTICATION

Implement the real current Pokémon Showdown login flow.

Support:

- guest play where the server permits it
- registered usernames
- password login
- restoring an authenticated session if safely possible
- logout
- failed login messaging

Do not invent your own authentication service.

Never log passwords, assertions, cookies, auth tokens or equivalent secrets.

Use OS keychain/keyring storage if feasible.

If secure persistence is unavailable, fail gracefully and require login again rather than writing plaintext secrets.

# MATCHMAKING

Provide a fast format picker.

Main flow should make Random Battle absurdly quick.

Examples:

`showdown rand`

should launch directly toward Gen 9 Random Battle after connection/authentication.

Support CLI conveniences such as:

`showdown`
`showdown rand`
`showdown play gen9ou`
`showdown spectate <battle-id-or-url>`

Inside the app provide fuzzy format search.

Support:

- ladder search
- cancelling search
- challenges
- accepting/rejecting challenges
- selecting teams for formats that require them
- multiple simultaneous battle rooms

# BATTLE UX

This is the most important part of the product.

Optimize for PLAYABILITY rather than information density.

Default full battle layout conceptually:

┌ showdown · Gen 9 Random Battle · Turn 11 ───────────────┐
│                                                         │
│        opponent name                                    │
│        Dragapult                            71% PAR      │
│                  [sprite]                               │
│                                                         │
│                         VS                              │
│                                                         │
│                  [sprite]                               │
│        Kingambit                            86%          │
│        your name                                        │
│                                                         │
│ Rain 3 · SR: yours · Reflect: opponent 2                │
├─────────────────────────────────────────────────────────┤
│ 1 Kowtow Cleave     DARK    85       16/16              │
│ 2 Sucker Punch      DARK    70        7/8               │
│ 3 Iron Head         STEEL   80       23/24              │
│ 4 Swords Dance      STATUS           31/32              │
├─────────────────────────────────────────────────────────┤
│ [1-4] move   [s] switch   [t] tera   [i] inspect       │
└─────────────────────────────────────────────────────────┘

Do not treat this exact ASCII layout as mandatory. Improve it.

Important UX goals:

- most singles turns: ONE keystroke
- common doubles turns: very few keystrokes
- no mouse required
- visible legal choices
- unavailable choices visibly disabled
- immediate feedback after choosing
- clear waiting state
- clear YOUR TURN state
- avoid accidental forfeits/destructive commands
- no giant persistent battle log
- no permanent chat pane stealing battle space

# KEYBOARD INTERACTION

Default philosophy:

`1`-`4`
    choose move

`s`
    open switch overlay

`s` then `1`-`6`
    switch directly

`t`
    toggle/use the current generation's main battle mechanic where appropriate, then select move

`i`
    inspect selected/current Pokémon

`l`
    battle log overlay

`c`
    battle chat overlay

`:`
    command palette

`?`
    contextual help

`Esc`
    close overlay / cancel current interaction

`q`
    should NOT instantly forfeit a live battle

Use a safe quit/forfeit flow.

All keys should be configurable.

# DOUBLES

Doubles support is mandatory, not a later enhancement.

Model each active slot independently.

When a move requires a target:

- present only legal targets
- provide direct numeric target shortcuts
- make ally targeting possible when legal
- handle spread/no-target moves automatically
- support separate choices for each active Pokémon
- show already-entered choices clearly
- allow editing choices before submission when protocol permits

The UI should not make doubles feel like completing a customs declaration form.

# TEAM PREVIEW

Implement full team preview interaction.

Keyboard-first ordering should be quick.

For formats where order matters, make rearranging/selecting intuitive.

# SWITCHING / FORCED SWITCHES

Distinguish:

- voluntary switching
- forced switching
- multiple forced switches in doubles/multi-active formats

Only show legal candidates.

# BATTLE MECHANICS

Correctly expose mechanic controls based on the battle request, not hardcoded assumptions.

Potential mechanics include:

- Terastallization
- Mega Evolution
- Z-Moves
- Dynamax
- generation-specific choices

If the request says the mechanic is unavailable, don't show an active control.

# RESPONSIVE DESIGN

The app must be intentionally responsive.

Do NOT simply squeeze the desktop layout.

Use semantic layout modes approximately like:

>= 100 columns:
    cinematic

70-99:
    standard

45-69:
    sidecar

<45:
    compact

Tune thresholds after testing.

Cinematic:
- spacious battlefield
- larger sprites
- richer team/status presentation

Standard:
- normal primary experience

Sidecar:
- designed specifically for coding beside it
- smaller sprites
- moves remain immediately accessible
- secondary information hidden behind overlays

Compact:
- gameplay first
- sprites may become tiny or disappear
- textual HP/status/moves always usable

At extremely small dimensions show a graceful "terminal too small" state rather than corrupting output.

# CODING SIDECAR EXPERIENCE

This is one of the core differentiators.

While waiting for opponent input, optionally collapse visual emphasis.

Example:

`◌ vs foo · waiting · T18`

When input arrives:

`● YOUR TURN · 02:29`

Provide optional:

- terminal bell
- OSC desktop notification where appropriate
- platform desktop notification if practical

Notifications must be configurable.

Never spam notifications while the battle pane is already focused if this can reasonably be detected.

# MULTIPLE BATTLES

Users can have more than one battle open.

Provide a lightweight battle switcher/tab strip.

Indicate:

- waiting battles
- battles requiring user input
- completed battles

A background battle that needs input should visibly notify the user.

# COMMAND PALETTE

Implement a fuzzy command palette invoked by `:`.

Examples of concepts:

- Queue Gen 9 Random Battle
- Queue Gen 9 OU
- Challenge user
- Accept challenge
- Join room
- Spectate battle
- Open teams
- Toggle sprites
- Toggle notifications
- Change theme
- Open logs
- Quit

Power users should be able to operate much of the app through it.

# SPRITE SYSTEM

This is a signature feature.

Build it as an abstraction owned by us rather than leaking a third-party image library through the UI.

Interface conceptually:

type Renderer interface {
    Capabilities() ...
    Render(...)
    Clear(...)
    Resize(...)
    Close(...)
}

Backends:

1. Kitty graphics
2. iTerm2 graphics
3. Sixel
4. Unicode truecolour half-block fallback
5. optional no-sprite renderer

Selection should be:

auto -> best reliable available backend

Allow config override:

sprites.mode = auto | kitty | iterm2 | sixel | blocks | none

DO NOT make application correctness dependent on advanced terminal pixel protocols.

The half-block renderer is not an embarrassing fallback. Make it look intentionally good.

For half-block mode:

- preserve transparency against the current terminal background where feasible
- fit sprite inside allotted cell rectangle
- preserve aspect ratio
- use truecolour foreground/background
- use upper/lower block glyphs intelligently
- optionally experiment with quarter-block/braille modes only if they demonstrably improve appearance

The app must still be beautiful using only half-block sprites.

# IMPORTANT GO-TERMIMG RULE

Current go-termimg contains useful support for:

- Kitty
- iTerm2
- Sixel
- halfblocks
- Bubble Tea integration
- protocol detection
- scaling
- async/stateful rendering
- tmux passthrough

Use or adapt it where robust.

However:

DO NOT make the production UI depend on its experimental Kitty Unicode-placeholder / virtual-placement functionality.

Use predictable reserved sprite rectangles in the TUI.

If go-termimg proves buggy in a specific backend, isolate/fork/replace the problematic adapter rather than abandoning the Go architecture.

We own the `sprites.Renderer` abstraction specifically so that individual implementations are replaceable.

# SPRITE ASSETS

Do not vendor thousands of Pokémon sprite files into the binary/repository by default.

Investigate the current Pokémon Showdown/Smogon asset endpoints.

Resolve a sprite from canonical Pokémon/form identifiers.

Download on demand and cache locally.

Need to support at least:

- front sprites
- back sprites where available/useful
- shiny variants
- forms
- gender variants where they materially differ

Use appropriate cache invalidation/versioning.

Show a graceful placeholder immediately if the image is still downloading.

Network sprite loading must never block battle input.

Be careful with licensing.

The Smogon sprite repository explicitly distinguishes its code licensing from Pokémon/community sprite rights. Do not claim we own or relicense sprite artwork.

Add a clear THIRD_PARTY / attribution notice.

Do not package assets into our release unless their redistribution status has been deliberately verified.

# SPRITE ANIMATION

Investigate animated GIF sprites.

If animation can be made reliable and cheap:

- decode/cached frames
- animate at source-appropriate timing
- pause/reduce animation when pane is unfocused or resource constrained
- don't redraw the entire TUI unnecessarily for every image frame

Provide config:

sprites.animate = true/false

If a specific terminal/backend does not animate cleanly, use a high-quality static frame rather than introducing flicker.

Gameplay correctness and clean rendering beat animation.

# IMAGE PERFORMANCE

Sprite loading and conversion must happen off the main UI path.

Cache:

- downloaded source assets
- decoded sprites where useful
- scaled/cell-target representations when worthwhile

Do not continuously rerender a sprite when neither sprite nor dimensions changed.

On resize, coalesce repeated resize events.

Sixel is substantially more expensive than Kitty/iTerm rendering, so avoid needless frame churn.

# BATTLE STATE DISPLAY

Always show:

- player names
- current Pokémon
- HP
- status
- fainted/alive state
- turn
- available moves/PP
- relevant move disabled state
- relevant battle mechanic
- timer/waiting state

Provide compact representation for:

- weather
- terrain
- hazards
- screens
- field/side effects
- boosts

Don't drown the player in Pokémon Showdown implementation details.

# INSPECT OVERLAY

`i` opens contextual information.

For own Pokémon:
- known moves
- HP
- status
- boosts
- item if known
- ability if known
- tera type if known
- relevant volatile conditions

For opponent:
- ONLY information legitimately revealed by the server/battle so far
- never infer hidden information as fact

This is a client, not a cheating assistant.

# BATTLE LOG

Battle history should be an overlay or secondary view.

Make it readable rather than showing raw protocol.

Highlight:

- moves
- damage
- statuses
- fainting
- switches
- weather/field changes

Allow scrolling/searching.

Raw protocol should only appear in debug mode.

# CHAT

Battle chat:

- accessible but not permanently dominant
- send messages
- render ordinary messages sensibly
- tolerate unsupported HTML/rich messages without crashing

PMs and basic rooms should work.

Do not attempt to clone every exotic web-client room widget.

# TEAM MANAGEMENT

Support the things necessary to actually play non-random formats.

At minimum:

- list stored teams
- import Showdown text
- export Showdown text
- edit team text in a practical terminal editor UI OR allow `$EDITOR`
- parse/serialize packed team format for server submission
- choose team when entering a format
- duplicate/delete/rename teams
- store locally

A fancy graphical drag-and-drop teambuilder is explicitly NOT required.

Terminal users have keyboards. Use them.

# CONFIG

Use a human-readable config file, e.g. TOML.

Support settings for:

- username
- theme
- keybindings
- sprite renderer mode
- sprite animation
- notifications
- preferred default format
- sound/bell
- compact mode preference
- debug logs

Never store passwords directly in config.

Provide sensible defaults so config is optional.

# THEMES

Ship several tasteful themes.

Examples:

- default dark
- light
- Game Boy inspired
- monochrome
- high contrast

Don't turn it into RGB gamer sludge.

Respect NO_COLOR where sensible.

# REPLAYS

At battle end:

- show winner/result
- allow returning to lobby/battle list
- provide replay URL when available
- allow copying/opening replay URL
- preserve local human-readable battle history if configured

# RECONNECTION

Network interruptions must not destroy the application.

Implement:

- reconnect with bounded backoff
- clear offline/reconnecting status
- resubscribe/rejoin relevant rooms where possible
- recover active battles where server behaviour allows it
- refresh state safely

Never send stale battle choices after reconnection.

Request IDs must be respected.

# DEBUGGING

Provide:

`showdown --debug`

Debug mode should log:

- sanitized protocol events
- renderer selected
- terminal capability detection
- reconnect events
- sprite cache behaviour

Never log secrets.

Prefer structured logs written to a file so they do not corrupt TUI rendering.

# RENDERING ROBUSTNESS

The terminal must be restored correctly after:

- normal quit
- Ctrl+C
- panic where recoverable
- network failure
- authentication failure

Avoid leaving:

- cursor hidden
- terminal in alternate screen
- mouse capture enabled
- graphics remnants everywhere

Test repeated open/close cycles.

Test resize repeatedly.

Test opening/closing overlays with sprites visible.

# TMUX

Detect tmux.

Use passthrough only where appropriate.

If native graphics prove unreliable inside the user's multiplexer, gracefully drop to half-block rendering rather than producing corrupt images.

A battle that looks slightly more pixelated is acceptable.

A terminal containing half of Charizard after exit is not.

# ACCESSIBILITY / TERMINAL COMPATIBILITY

Gameplay cannot depend on colour alone.

Statuses/mechanics need textual indicators.

Support disabling sprites entirely.

Make UI usable with ordinary terminal fonts.

Avoid Unicode glyphs whose width behaviour is notoriously inconsistent when a safer alternative exists.

# PERFORMANCE TARGETS

This app should feel instantaneous.

Targets, not synthetic vanity benchmarks:

- keyboard response appears immediate
- normal events should not cause visible whole-screen flicker
- resizing should remain interactive
- sprite loading must not freeze gameplay
- idle CPU use should remain very low
- waiting battles should not redraw continuously
- memory use should remain modest

Profile before making absurd micro-optimizations.

# CLI

Implement useful flags/subcommands.

Examples:

`showdown`
`showdown rand`
`showdown play gen9ou`
`showdown challenge <user>`
`showdown spectate <battle>`
`showdown teams`
`showdown --no-sprites`
`showdown --sprites=blocks`
`showdown --debug`
`showdown --version`

Exact syntax may improve during implementation.

# INSTALLATION / RELEASES

Provide:

- downloadable GitHub release binaries
- checksums
- Homebrew installation path/formula instructions
- `go install` path where practical

Use GoReleaser.

Automate release builds.

README should include:

- a GIF/video/screenshots of actual TUI
- install
- first run
- controls
- terminal compatibility
- sprite backend explanation
- troubleshooting
- tmux note
- privacy/authentication note
- asset/licensing attribution
- contributing instructions

# TESTING

Build substantial tests.

Unit tests:
- protocol parser
- battle reducer
- choice construction
- packed teams
- sprite resolution
- renderer capability selection
- responsive layout mode selection
- config

Golden/snapshot tests:
- important TUI states
- compact/sidecar/standard layouts
- battle request rendering

Integration tests:
- recorded full battle streams
- mock websocket server
- reconnect
- challenges/search updates
- authentication request construction without exposing credentials

Renderer tests:
- encoded output sanity
- half-block pixel conversion
- transparency behaviour
- scaling/aspect ratio
- cache
- resize

Use race detector where possible.

Run:
- go test ./...
- go test -race ./...
- go vet ./...
- staticcheck or equivalent
- formatting checks

# MANUAL TERMINAL TEST MATRIX

Document and execute as much as the environment permits.

At minimum design for:

Ghostty:
- native
- tmux

Kitty:
- native
- tmux

iTerm2:
- native
- tmux

WezTerm:
- native
- tmux

Generic ANSI:
- half-block

For each check:

- startup
- sprite appears in correct rectangle
- resize
- overlay over battle
- close overlay
- switch Pokémon
- sprite changes
- terminal restored after quit
- no stale graphics
- compact mode
- full mode

If automated environment cannot launch real GUI terminal emulators, create executable diagnostics and document the remaining manual checks clearly rather than pretending they passed.

# GRAPHICS DIAGNOSTIC

Ship something like:

`showdown doctor`

It should report:

- terminal name
- TERM
- multiplexer detection
- truecolour support
- detected graphics protocols
- selected sprite backend
- cell pixel dimensions if measurable
- config/cache paths
- notification availability

Optionally include:

`showdown doctor --sprites`

to display a test sprite through each supported renderer.

This will be invaluable for GitHub bug reports.

# PRODUCT POLISH

Add little details that make this pleasant:

- proper loading states
- no flickering spinners everywhere
- contextual status line
- tasteful transitions using text/state rather than gratuitous animation
- clear disconnected state
- readable errors
- confirmation before destructive actions
- command palette history
- keybinding hints that disappear in compact layouts
- remembered preferred format
- sensible focus behaviour
- clipboard support where available

# NON-GOALS

Do NOT waste time cloning:

- Showdown's news/homepage
- Smogon forums
- arbitrary website panels
- every custom HTML chat-room widget
- drag-and-drop team building
- hover-based browser tooltips
- ads
- browser UI conventions

We're building a battle-centric native terminal client.

# SECURITY

Treat server messages as untrusted input.

- sanitize terminal control sequences from chat/usernames/server text before rendering
- never allow another user to inject arbitrary ANSI escape codes
- cap unreasonable payload sizes
- validate downloaded asset types/sizes
- use HTTPS/WSS
- don't execute remote content
- don't log secrets
- don't shell interpolate usernames/battle IDs/etc.

This is especially important in a terminal application.

# QUALITY BAR

Don't create abstractions simply because architecture diagrams enjoy rectangles.

Prefer idiomatic, readable Go.

Avoid giant interfaces.

Add comments where protocol behaviour is non-obvious.

Do not prematurely fork dependencies.

But also do not contort the entire product around a dependency bug. Our domain and renderer boundaries should allow replacement.

# ACCEPTANCE CRITERIA

Do NOT call the project complete until all of these are true:

1. Application launches into a polished TUI.
2. User can connect to the real Pokémon Showdown service.
3. User can authenticate.
4. User can retrieve current battle formats.
5. User can search for a Random Battle.
6. Match appears without opening a browser.
7. Pokémon sprites render.
8. At least Kitty, iTerm2/Sixel where available, and half-block fallback are architecturally supported.
9. Half-block rendering is polished enough to be an acceptable default fallback.
10. User can complete an entire real singles battle.
11. User can complete an entire real doubles battle.
12. Team preview works.
13. Forced switches work.
14. Modern mechanics exposed by Showdown requests work.
15. Timer state works.
16. Battle log works.
17. Battle chat works.
18. User can forfeit safely.
19. Challenges work.
20. Spectating works.
21. User can select/import/export local teams.
22. Multiple battles can coexist.
23. Background battle turn notifications work.
24. Sidecar/narrow layout is genuinely playable.
25. Compact layout remains functional without sprites.
26. Terminal resizing does not corrupt the screen.
27. Temporary WebSocket disconnection does not require restarting the program in ordinary cases.
28. Terminal is restored cleanly on exit.
29. `showdown doctor` exists.
30. README/install/release configuration exists.
31. Tests cover protocol and battle-state behaviour meaningfully.
32. CI passes.
33. Release binaries can be generated.
34. No known crash exists in ordinary supported battle flows.
35. No password/token leakage exists in logs/config.
36. Unknown protocol events fail gracefully.
37. The codebase is maintainable enough that someone unfamiliar with it can add a new protocol event or component without archaeology.

# DEFINITION OF DONE

"Works on my terminal" is not done.

"Can display Pikachu" is not done.

"Can connect and parse messages" is not done.

"Singles mostly works" is not done.

A complete, battle-capable, distributable open-source client is done.

When facing ambiguity, make sensible product/engineering decisions instead of stopping for trivial approval.

When a third-party library does not behave as required, diagnose it and implement the smallest reliable adapter/workaround necessary.

Keep a running `docs/IMPLEMENTATION_NOTES.md` documenting important protocol discoveries, renderer compromises and manual tests.

Before declaring completion, perform a final review of:

- functionality
- UX
- protocol correctness
- rendering
- terminal cleanup
- tests
- security
- packaging
- documentation

Then provide a concise final report containing:

- what was built
- architecture
- how to run it
- what was tested
- terminal compatibility
- any genuinely unavoidable remaining limitations

Do not substitute a roadmap for implementation.