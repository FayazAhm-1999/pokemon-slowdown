package battle

import "strings"

// Choice is a complete decision for one request: one slot per active Pokémon,
// or a single team-preview / undo / default token.
type Choice struct {
	Slots []ChoiceSlot
	// Raw overrides slot rendering entirely (used for "undo" and "default").
	Raw string
}

// String renders the choice in protocol syntax, e.g.
// "move 1 +1, move 3 terastalize".
func (c Choice) String() string {
	if c.Raw != "" {
		return c.Raw
	}
	parts := make([]string, 0, len(c.Slots))
	for _, s := range c.Slots {
		parts = append(parts, s.String())
	}
	return strings.Join(parts, ", ")
}

// Empty reports whether the choice has no decisions yet.
func (c Choice) Empty() bool { return c.Raw == "" && len(c.Slots) == 0 }

// UndoChoice cancels a previously submitted choice where the server allows it.
func UndoChoice() Choice { return Choice{Raw: "undo"} }

// DefaultChoice asks the server to auto-select a legal decision.
func DefaultChoice() Choice { return Choice{Raw: "default"} }

// TeamChoice builds a team-preview choice from a 1-based order of party slots.
func TeamChoice(order []int) Choice {
	parts := make([]string, 0, len(order))
	for _, n := range order {
		parts = append(parts, itoa(n))
	}
	return Choice{Raw: "team " + strings.Join(parts, "")}
}

// ---------------------------------------------------------------------------
// Request helpers
// ---------------------------------------------------------------------------

// SlotCount is the number of decisions the request requires.
func (r *Request) SlotCount() int {
	switch r.Kind() {
	case RequestMove:
		return len(r.Active)
	case RequestSwitch:
		return len(r.ForceSwitch)
	case RequestTeam:
		return 1
	default:
		return 0
	}
}

// MustSwitchAt reports whether the active slot at index i must switch out.
func (r *Request) MustSwitchAt(i int) bool {
	if r.Kind() != RequestSwitch {
		return false
	}
	if i < 0 || i >= len(r.ForceSwitch) {
		return false
	}
	return r.ForceSwitch[i]
}

// ActiveAt returns the move request for active slot i, or nil.
func (r *Request) ActiveAt(i int) *PokemonMoveRequest {
	if i < 0 || i >= len(r.Active) {
		return nil
	}
	return &r.Active[i]
}

// BenchSlots returns the 1-based party slots that may be switched in. Only
// healthy, inactive Pokémon are legal.
func (r *Request) BenchSlots() []int {
	var out []int
	for i, p := range r.Side.Pokemon {
		if p.Active || conditionFainted(p.Condition) {
			continue
		}
		out = append(out, i+1)
	}
	return out
}

// AllSwitchSlots returns every 1-based party slot with a fainted marker.
type SwitchSlot struct {
	Index   int // 1-based party slot
	Pokemon PokemonSwitchRequest
	Fainted bool
	Active  bool
	Legal   bool
}

// SwitchSlots returns the whole party annotated for a switch overlay.
func (r *Request) SwitchSlots() []SwitchSlot {
	out := make([]SwitchSlot, 0, len(r.Side.Pokemon))
	for i, p := range r.Side.Pokemon {
		fainted := conditionFainted(p.Condition)
		out = append(out, SwitchSlot{
			Index:   i + 1,
			Pokemon: p,
			Fainted: fainted,
			Active:  p.Active,
			Legal:   !fainted && !p.Active,
		})
	}
	return out
}

func conditionFainted(cond string) bool {
	_, status := splitHP(cond)
	if status == "fnt" {
		return true
	}
	hp, _ := splitHP(cond)
	cur, max, _ := parseHP(hp)
	return max > 0 && cur <= 0
}
