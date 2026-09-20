# Third-party notices

`pokemon slowdown` is an independent, unofficial client. It is not affiliated
with, endorsed by, or sponsored by The Pokémon Company, Nintendo, Creatures
Inc., GAME FREAK inc., Smogon, or Pokémon Showdown.

## Pokémon and sprite artwork

Pokémon and Pokémon character names are trademarks of Nintendo, Creatures Inc.
and GAME FREAK inc.

Sprite artwork is **not** distributed with this project and is **not** covered
by its MIT licence. Sprites are downloaded at runtime from the public Pokémon
Showdown sprite server (`https://play.pokemonshowdown.com/sprites/`) into a
local cache directory, and remain subject to whatever rights their authors and
The Pokémon Company hold. The Pokémon Showdown project explicitly distinguishes
the licensing of its *code* from the rights in Pokémon and community sprite
artwork. This project claims no ownership of, and does not relicense, that
artwork.

If you are a rights holder and want a sprite removed from the upstream server,
that request belongs with the Pokémon Showdown project, not with this client.

## Reference data

Species and move reference data (`pokedex.json`, `moves.json`) is fetched at
runtime from the same public server and cached locally. It is used only to
display types, base stats, move categories and sprite identifiers.

## Protocol

The Pokémon Showdown wire protocol is implemented independently from the
publicly documented protocol in the `smogon/pokemon-showdown` repository
(`PROTOCOL.md`, `sim/SIM-PROTOCOL.md`, `sim/TEAMS.md`). No client source code
from Pokémon Showdown's AGPL-licensed web client was copied into this project.

## Go dependencies

| Module | Licence |
| --- | --- |
| `charm.land/bubbletea/v2` | MIT |
| `charm.land/lipgloss/v2` | MIT |
| `charm.land/bubbles/v2` | MIT |
| `github.com/coder/websocket` | ISC |
| `github.com/BurntSushi/toml` | MIT |
| `github.com/sahilm/fuzzy` | MIT |
| `github.com/zalando/go-keyring` | MIT |
| `github.com/mattn/go-sixel` | MIT |
| `github.com/blacktop/go-termimg` | MIT (inspiration and protocol reference) |

Full dependency licences are available in each module's repository and in the
Go module cache.
