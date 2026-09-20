package sprites

// BaseURL is the public Pokémon Showdown sprite server. It is a variable so
// tests can point the resolver at a local server.
var BaseURL = "https://play.pokemonshowdown.com/sprites"

// Ref identifies a sprite to fetch.
type Ref struct {
	// ID is the sprite filename stem, e.g. "gengar" or "landorus-therian".
	ID string
	// Shiny selects the shiny variant.
	Shiny bool
	// Back selects the back sprite.
	Back bool
	// Animated prefers an animated GIF where one exists.
	Animated bool
}

// Key returns a stable cache key for the reference.
func (r Ref) Key() string {
	kind := "front"
	if r.Back {
		kind = "back"
	}
	if r.Shiny {
		kind = "shiny-" + kind
	}
	if r.Animated {
		kind += "-ani"
	}
	return r.ID + "|" + kind
}

// Candidates returns the URLs to try, best first. Fallbacks keep the client
// useful when a particular variant has not been published: an animated shiny
// back sprite falls back to the static shiny back, then the plain back, then
// the front sprite, and finally the dex art.
func Candidates(ref Ref) []string {
	id := ref.ID
	if id == "" {
		return nil
	}
	var out []string
	add := func(dir, ext string) {
		out = append(out, BaseURL+"/"+dir+"/"+id+ext)
	}

	if ref.Animated {
		if ref.Back {
			if ref.Shiny {
				// ani-shiny-back does not exist upstream; fall through to
				// static shiny back, then animated back.
				add("ani-back", ".gif")
			}
			add("ani-back", ".gif")
		} else {
			if ref.Shiny {
				add("ani-shiny", ".gif")
			}
			add("ani", ".gif")
		}
	}

	if ref.Back {
		if ref.Shiny {
			add("gen5-back-shiny", ".png")
		}
		add("gen5-back", ".png")
	}
	if ref.Shiny {
		add("gen5-shiny", ".png")
	}
	add("gen5", ".png")
	add("dex", ".png")
	return dedupe(out)
}

func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}
