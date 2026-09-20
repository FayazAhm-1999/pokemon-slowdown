package battle

// TargetOption is one selectable target for a move.
type TargetOption struct {
	// Spec is the protocol target spec: "+1", "-2", or "" when the move needs
	// no explicit target.
	Spec string
	// Foe is true when the target is on the opposing side.
	Foe bool
	// Slot is the 0-based index within the foe or ally side.
	Slot int
}

// NeedsTarget reports whether a move requires an explicit target given how many
// Pokémon are active on the user's side. Singles never needs one.
func NeedsTarget(m MoveRequest, activeCount int) bool {
	if activeCount < 2 {
		return false
	}
	switch m.Target {
	case "normal", "any", "adjacentAlly", "adjacentAllyOrSelf":
		return true
	}
	return false
}

// LegalTargets returns the legal targets for a move used from active slot
// `slot`. myCount and foeCount are the numbers of active Pokémon on each side.
func LegalTargets(m MoveRequest, slot, myCount, foeCount int) []TargetOption {
	foes := make([]TargetOption, 0, foeCount)
	for i := 0; i < foeCount; i++ {
		foes = append(foes, TargetOption{Spec: "+" + itoa(i+1), Foe: true, Slot: i})
	}
	allies := func(includeSelf bool) []TargetOption {
		out := make([]TargetOption, 0, myCount)
		for i := 0; i < myCount; i++ {
			if i == slot && !includeSelf {
				continue
			}
			out = append(out, TargetOption{Spec: "-" + itoa(i+1), Slot: i})
		}
		return out
	}

	switch m.Target {
	case "normal":
		return foes
	case "any":
		return append(foes, allies(false)...)
	case "adjacentAlly":
		return allies(false)
	case "adjacentAllyOrSelf":
		return allies(true)
	default:
		// Spread, self, side and scripted moves take no explicit target.
		return nil
	}
}

// AutoTarget returns the default target for a move when the UI wants a single
// keystroke to resolve it, or "" when the move needs no target.
func AutoTarget(m MoveRequest, slot, myCount, foeCount int) string {
	targets := LegalTargets(m, slot, myCount, foeCount)
	if len(targets) == 0 {
		return ""
	}
	return targets[0].Spec
}
