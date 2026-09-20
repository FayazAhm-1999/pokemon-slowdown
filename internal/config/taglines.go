package config

import "math/rand"

// TaglineSlot identifies where a tagline is shown.
type TaglineSlot string

// Tagline slots.
const (
	SlotSplash       TaglineSlot = "splash"
	SlotWaiting      TaglineSlot = "waiting"
	SlotDisconnected TaglineSlot = "disconnected"
	SlotYourTurn     TaglineSlot = "your_turn"
	SlotVictory      TaglineSlot = "victory"
	SlotDefeat       TaglineSlot = "defeat"
	SlotIdle         TaglineSlot = "idle"
	SlotExit         TaglineSlot = "exit"
	SlotEasterEgg    TaglineSlot = "easter_egg"
)

// canonTaglines are lines taken from the games and the anime. They are the
// default voice of the client: quiet, a little deadpan, never marketing copy.
var canonTaglines = map[TaglineSlot][]string{
	SlotSplash: {
		"There's a time and place for everything",
		"Now is the time to choose!",
		"Welcome to the world of Pokémon!",
	},
	SlotWaiting: {
		"Somewhere, your opponent is staring at a screen.",
		"Please wait.",
	},
	SlotDisconnected: {
		"Smell ya later.",
	},
	SlotYourTurn: {
		"What will you do?",
	},
	SlotVictory: {
		"Congratulations! You won!",
		"That was a great battle!",
	},
	SlotDefeat: {
		"You whited out!",
		"You have no more Pokémon that can fight!",
	},
	SlotIdle: {
		"The Pokémon Center is a place to rest.",
		"zzz…",
	},
	SlotExit: {
		"We hope to see you again!",
		"Please come again!",
	},
	SlotEasterEgg: {
		"I like shorts! They're comfy and easy to wear!",
		"My Rattata is in the top percentage of Rattata.",
		"It's not the time to use that!",
		"The ROAD is closed because of a landslide.",
	},
}

// absurdTaglines are affectionate riffs in the same voice, for people who want
// the client to be a little stranger.
var absurdTaglines = map[TaglineSlot][]string{
	SlotSplash: {
		"A wild TUI appeared!",
		"Six Pokémon. Zero browser tabs.",
		"There's a time and place for everything. This is the place.",
	},
	SlotWaiting: {
		"Your opponent is pondering the orb.",
		"Somewhere, a Snorlax is still asleep.",
	},
	SlotDisconnected: {
		"The tall grass rustled, and then went quiet.",
	},
	SlotYourTurn: {
		"The fate of six small creatures rests on your keyboard.",
	},
	SlotVictory: {
		"Your Pokémon are mildly impressed.",
	},
	SlotDefeat: {
		"Your Pokémon need a nap, and honestly, so do you.",
	},
	SlotIdle: {
		"Snorlax used Rest.",
		"No thoughts. Just Haze.",
	},
	SlotExit: {
		"Come back when you have more badges.",
	},
	SlotEasterEgg: {
		"The ROAD is closed because of a landslide.",
		"A wild Rattata appeared! It is in the top percentage.",
	},
}

// Tagline returns a line for a slot according to the configured mode. "off"
// returns "", and random rotates through the pool.
func (c Config) Tagline(slot TaglineSlot, random bool) string {
	pool := c.taglinePool(slot)
	if len(pool) == 0 {
		return ""
	}
	if random {
		return pool[rand.Intn(len(pool))]
	}
	return pool[0]
}

func (c Config) taglinePool(slot TaglineSlot) []string {
	switch c.Taglines {
	case "off":
		return nil
	case "absurd":
		if p := absurdTaglines[slot]; len(p) > 0 {
			return p
		}
		return canonTaglines[slot]
	default:
		return canonTaglines[slot]
	}
}

// EasterEgg returns a random easter-egg line, or "" when taglines are off.
func (c Config) EasterEgg() string {
	pool := c.taglinePool(SlotEasterEgg)
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))]
}
