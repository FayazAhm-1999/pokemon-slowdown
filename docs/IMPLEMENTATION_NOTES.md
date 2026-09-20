# Implementation notes

Working notes on protocol behaviour, rendering compromises and manual testing.
Kept because these are the things that are expensive to rediscover.

## Protocol discoveries

These were verified against upstream documentation and, where the docs were
silent, against `sim/` source in `smogon/pokemon-showdown`.

### Framing

- Server payloads may batch several rooms. A line starting with `>` begins a
  new room; everything before the first `>` is global.
- Empty lines inside a frame are not messages and must be ignored.
- Chat messages can contain `|`, so `|c|user|message` must be split with a
  bounded split (four fields, remainder kept whole). The same applies to
  `|challstr|`, `|formats|`, `|request|`, `|pm|`, `|updatesearch|`,
  `|updatechallenges|`, `|queryresponse|` and `|updateuser|`.

### Tag detection

Protocol tags (`[from] item: Life Orb`, `[miss]`, `[still]`, `[upkeep]`) are
distinguishable from ordinary arguments by their key being a **lowercase
identifier**. A naive "starts with `[`" test is wrong: the tier line
`|tier|[Gen 9] Random Battle` looks like a tag and silently swallows the format
name. `isTagArg` requires the bracketed key to be `[a-z-]+`.

### HP and status

- `HP` and `STATUS` share a single field: `|switch|p1a: X|X, L82, M|241/241`
  and `|-damage|p1a: X|50/100 brn`. Split on the first space.
- Own Pokémon report `current/max`; opponents report a percentage (`62%`) or
  `/48` without the HP Percentage Mod.
- `0 fnt` means the Pokémon fainted; `0/227` also means fainted. Both paths
  set the fainted flag.

### Users

`|users|` entries are `USER` values where the rank is a leading non-alphanumeric
character. `@` is *both* a rank prefix and the status separator (`Bob@!away`),
so only a `@` at index > 0 delimits a status.

### Formats

The `|formats|` payload is a flat field list. A field that is empty or starts
with a comma followed by a number is a **section marker**; the *next* field is
the section name. Everything else is a format entry.

Entries are **display names**, not ids. The id is `toID(name)`, so
`[Gen 9] Random Battle` is `gen9randombattle`.

The documented `,#` / `,,` / `,` suffix form is obsolete. The live server
appends a comma and a **hex flag bitmask**, decoded from the reference client's
`panel-mainmenu.tsx`:

| Bit | Meaning |
| --- | --- |
| 1 | preset team (Random Battle and friends) |
| 2 | searchable on the ladder |
| 4 | challengeable |
| 8 | usable in tournaments |
| 16 | teambuilder level 50 |
| 32 | partner format |
| 64 | best-of default |
| 128 | tera preview default |
| 256 | item clause default |

So `[Gen 9] Random Battle,4f` is `0x4f` = preset + searchable + challengeable +
tournament + best-of, and `[Gen 9] Custom Game,c` is `0x0c` = challengeable +
tournament only. Parsing this with the documented suffix rules instead of the
bitmask mislabels nearly every format. Both forms are now accepted, the legacy
one only as a fallback.

Because names arrive with the entries, the UI prefers the server's name and
falls back to the curated table only for ids that arrive without one.

### Requests

Decoded from `sim/side.ts`:

- `canMegaEvo`, `canMegaEvoX`, `canMegaEvoY`, `canUltraBurst`
- `canZMove` — an array parallel to `moves`, each `null` or `{move, target}`
- `canDynamax` plus `maxMoves: {maxMoves: [{move, target, disabled}], gigantamax}`
- `canTerastallize` — a tera type string
- `disabled` is `string | boolean`, so it needs a custom unmarshaller
- `forceSwitch: []bool` marks a forced switch request
- Team preview is `teamPreview: true`; a wait is `wait: true`

Mechanics are derived entirely from the request. A control is only shown when
the server says the mechanic is available, so nothing is hardcoded per
generation.

### Terastallization

The protocol message is `|-terastallize|POKEMON|TYPE`
(`battle.add('-terastallize', pokemon, type)` in `sim/battle-actions.ts`). It is
a minor action, so it arrives through the generic effect path.

### Choices

- `/choose CHOICE|RQID` — the request id must be echoed back, otherwise "undo"
  can apply a decision to the wrong turn.
- Doubles decisions are comma-separated. Targets are `+N` for foes and `-N` for
  allies, 1-based. Singles never needs a target.
- `default`, `undo`, `pass` and `team 213456` are all valid choices.

## Sprite naming

Showdown's sprite filenames are *not* simply `toID(species)`. The rule that
matches the published files is:

- forme: `toID(baseSpecies) + "-" + toID(forme)` → `landorus-therian`,
  `charizard-megax`, `urshifu-rapidstrike`, `meowstic-f`
- otherwise: `toID(name)` → `hooh`, `porygonz`, `nidoranf`, `mrmime`, `tinglu`

Confirmed against `pokedex.json` (`baseSpecies` / `forme`) and by probing the
sprite server. `sprites.Candidates` still tries several fallbacks, so a missing
variant degrades to the static shiny back, then the plain back, then the front
sprite, then the dex art.

Directories verified to exist: `gen5`, `gen5-shiny`, `gen5-back`,
`gen5-back-shiny`, `ani`, `ani-shiny`, `ani-back`, `dex`.
`ani-shiny-back` does **not** exist.

## Rendering compromises

### Half-block is the default, deliberately

Bubble Tea's renderer diffs the screen. Graphics escape sequences embedded in a
view string are at the mercy of that diffing: an unchanged line is not
re-emitted, so an image can survive correctly, but any redraw of the region can
leave artefacts, and there is no way to verify this across every terminal from
inside the test suite.

Given the explicit requirement that *"a terminal containing half of Charizard
after exit is not acceptable"*, `auto` selects the half-block backend, which is
pure text and therefore immune to the problem. Kitty, iTerm2 and sixel backends
are implemented and reachable via `sprites.mode`, and
`slowdown doctor --sprites` renders through each so a user can verify before
committing. Inside tmux, `auto` always uses half-blocks.

The half-block renderer composites transparency rather than painting it: a cell
with one transparent half uses a single half-block glyph and never sets a
background colour, so the terminal background shows through.

### Reserved rectangles

Sprites always occupy a fixed cell rectangle per layout mode. A placeholder is
drawn immediately and replaced when the image arrives, so a slow download never
reflows the battle UI.

### Animation

Animated GIFs are decoded into frames with their delays. Animation advances on
the existing 120ms UI tick and only re-renders sprites in the focused battle, at
roughly 8fps. Waiting battles do not redraw continuously.

## Manual terminal test matrix

Automated tests cover protocol parsing, the reducer, choice construction, packed
teams, sanitization, layout selection and rendering at many widths. They cannot
verify what a real terminal *displays*, so the following remains manual.

Run `slowdown doctor --sprites` first: it renders a Pikachu through each backend
so you can see which ones your terminal actually honours.

| Check | Ghostty | Kitty | iTerm2 | WezTerm | tmux | generic ANSI |
| --- | --- | --- | --- | --- | --- | --- |
| startup | | | | | | |
| sprite in correct rectangle | | | | | | |
| resize repeatedly | | | | | | |
| overlay over battle | | | | | | |
| close overlay | | | | | | |
| switch Pokémon, sprite changes | | | | | | |
| terminal restored after quit | | | | | | |
| no stale graphics | | | | | | |
| compact mode | | | | | | |
| cinematic mode | | | | | | |

Development and automated verification happened in Ghostty
(`TERM_PROGRAM=ghostty`). The other columns have not been verified on real
hardware in this environment and are listed rather than claimed.

## Testing against the live service

Two opt-in suites run against the real world:

```sh
SLOWDOWN_LIVE=1 go test ./internal/showdown/ -run TestLive -v   # endpoint, login, formats
SLOWDOWN_LIVE=1 go test ./internal/sprites/  -run TestLive -v   # real sprite downloads and rendering
```

The sprite suite is the one that matters for the renderer. It downloads real
sprites and:

- decodes front, shiny, back and forme sprites (96×96 PNGs)
- decodes an animated GIF (Gengar: 39 frames)
- renders each sprite with the half-block backend, **parses the ANSI back into
  pixels**, and compares them against the source image — currently 100% of
  opaque pixels round-trip, with 11–14 distinct colours, so a blank or flat box
  fails the test
- checks transparency survives as real spaces
- base64-decodes the Kitty payload and asserts it is a valid PNG at the source
  dimensions, and that control data appears exactly once across chunks
- base64-decodes the iTerm2 payload the same way
- asserts the sixel output is real DCS data

Verified results at the time of writing: 8/8 species decode, 213/213, 327/327
and 368/368 opaque pixels matched for Gengar, Ting-Lu and Landorus-Therian.

## Known limitations

- Sprite animation quality depends on GIF frame disposal; frames are used
  directly rather than composited, which is correct for the upstream assets but
  would need revisiting for other sources.
- The format display-name table is curated. Unknown ids fall back to a
  heuristic rather than being fetched, because the upstream names live in a
  JavaScript file rather than a JSON endpoint.
- Only singles and doubles target selection are modelled in the UI; triples,
  multi and free-for-all parse correctly but their target pickers are not
  tailored.
- Chat rooms render as plain text. Exotic HTML room widgets are explicitly out
  of scope.
