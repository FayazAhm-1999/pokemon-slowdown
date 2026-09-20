package battle

import (
	"bytes"
	"encoding/json"
)

// RequestKind classifies a choice request.
type RequestKind string

// Request kinds.
const (
	RequestWait   RequestKind = "wait"
	RequestTeam   RequestKind = "team"
	RequestSwitch RequestKind = "switch"
	RequestMove   RequestKind = "move"
)

// Request is a decoded |request| payload.
type Request struct {
	Wait        bool                 `json:"wait"`
	TeamPreview bool                 `json:"teamPreview"`
	ForceSwitch []bool               `json:"forceSwitch"`
	Active      []PokemonMoveRequest `json:"active"`
	Side        SideRequest          `json:"side"`
	Ally        *SideRequest         `json:"ally"`
	RqID        int                  `json:"rqid"`
	NoCancel    bool                 `json:"noCancel"`
	MaxChosen   int                  `json:"maxChosenTeamSize"`
	Update      bool                 `json:"update"`
}

// Kind classifies the request. Order matters: a team preview is also a switch
// request, and a wait request carries no actionable choice.
func (r *Request) Kind() RequestKind {
	switch {
	case r == nil:
		return RequestWait
	case r.Wait:
		return RequestWait
	case r.TeamPreview:
		return RequestTeam
	case len(r.ForceSwitch) > 0:
		return RequestSwitch
	default:
		return RequestMove
	}
}

// NeedsChoice reports whether the server is waiting on us.
func (r *Request) NeedsChoice() bool { return r.Kind() != RequestWait }

// PokemonMoveRequest describes the choices available to one active Pokémon.
type PokemonMoveRequest struct {
	Moves           []MoveRequest   `json:"moves"`
	Trapped         bool            `json:"trapped"`
	MaybeTrapped    bool            `json:"maybeTrapped"`
	MaybeDisabled   bool            `json:"maybeDisabled"`
	MaybeLocked     bool            `json:"maybeLocked"`
	CanMegaEvo      bool            `json:"canMegaEvo"`
	CanMegaEvoX     bool            `json:"canMegaEvoX"`
	CanMegaEvoY     bool            `json:"canMegaEvoY"`
	CanUltraBurst   bool            `json:"canUltraBurst"`
	CanZMove        []*ZMoveRequest `json:"canZMove"`
	CanDynamax      bool            `json:"canDynamax"`
	MaxMoves        *DynamaxOptions `json:"maxMoves"`
	CanTerastallize string          `json:"canTerastallize"`
}

// MoveRequest describes one move available to an active Pokémon.
type MoveRequest struct {
	Move           string   `json:"move"`
	ID             string   `json:"id"`
	PP             int      `json:"pp"`
	MaxPP          int      `json:"maxpp"`
	Target         string   `json:"target"`
	Disabled       Disabled `json:"disabled"`
	DisabledSource string   `json:"disabledSource"`
}

// ZMoveRequest is the Z-Move form of a move.
type ZMoveRequest struct {
	Move   string `json:"move"`
	Target string `json:"target"`
}

// DynamaxOptions is the Dynamax form of a move list.
type DynamaxOptions struct {
	MaxMoves   []MaxMoveRequest `json:"maxMoves"`
	Gigantamax string           `json:"gigantamax"`
}

// MaxMoveRequest is one Dynamax move.
type MaxMoveRequest struct {
	Move     string `json:"move"`
	Target   string `json:"target"`
	Disabled bool   `json:"disabled"`
}

// Disabled is the protocol's "disabled" field, which may be a boolean or a
// string describing why the move is unavailable.
type Disabled struct {
	Set    bool
	Reason string
}

// UnmarshalJSON accepts a bool, a string, or null.
func (d *Disabled) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	switch b[0] {
	case 't':
		d.Set = true
	case 'f':
		d.Set = false
	default:
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		d.Set = s != ""
		d.Reason = s
	}
	return nil
}

// SideRequest describes our whole team.
type SideRequest struct {
	Name     string                 `json:"name"`
	ID       string                 `json:"id"`
	Pokemon  []PokemonSwitchRequest `json:"pokemon"`
	NoCancel bool                   `json:"noCancel"`
}

// PokemonSwitchRequest describes one Pokémon on our team.
type PokemonSwitchRequest struct {
	Ident       string         `json:"ident"`
	Details     string         `json:"details"`
	Condition   string         `json:"condition"`
	Active      bool           `json:"active"`
	Stats       map[string]int `json:"stats"`
	Moves       []string       `json:"moves"`
	BaseAbility string         `json:"baseAbility"`
	Item        string         `json:"item"`
	Pokeball    string         `json:"pokeball"`
	Ability     string         `json:"ability"`
	Commanding  bool           `json:"commanding"`
	Reviving    bool           `json:"reviving"`
}

// ParseRequest decodes a raw |request| JSON payload.
func ParseRequest(raw string) (*Request, error) {
	var r Request
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// ChoiceSlot is one Pokémon's decision within a request.
type ChoiceSlot struct {
	// Kind is "move", "switch", "pass" or "default".
	Kind string
	// Move is the 1-based move slot (1-4).
	Move int
	// Switch is the 1-based party slot (1-6).
	Switch int
	// Target is the protocol target spec ("+1", "-2", ""). Empty means the
	// move needs no explicit target.
	Target string
	// Mechanic is "", "mega", "ultraburst", "zmove", "max" or "terastalize".
	Mechanic string
}

// String renders the slot in protocol choice syntax.
func (c ChoiceSlot) String() string {
	switch c.Kind {
	case "pass":
		return "pass"
	case "default":
		return "default"
	case "switch":
		return "switch " + itoa(c.Switch)
	case "move":
		s := "move " + itoa(c.Move)
		if c.Target != "" {
			s += " " + c.Target
		}
		if c.Mechanic != "" {
			s += " " + c.Mechanic
		}
		return s
	}
	return "default"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
