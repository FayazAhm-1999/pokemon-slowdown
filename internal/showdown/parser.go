package showdown

import (
	"strconv"
	"strings"
)

// Parse turns one frame into typed events. Lines that do not match any known
// message type are preserved as BattleEffect, BattleUnknown or RoomMessage
// rather than being dropped, so protocol drift is visible instead of fatal.
func Parse(f Frame) []Event {
	evs := make([]Event, 0, len(f.Lines))
	for _, line := range f.Lines {
		if ev, ok := parseLine(f.RoomID, line); ok {
			evs = append(evs, ev)
		}
	}
	return evs
}

func parseLine(room, line string) (Event, bool) {
	b := Base{RoomID: room}

	switch {
	case line == "|":
		return BattleClear{Base: b}, true
	case !strings.HasPrefix(line, "|"):
		return RoomMessage{Base: b, Message: line}, true
	}

	head := splitN(line, 3)
	if len(head) < 2 {
		return nil, false
	}
	typ := head[1]
	payload := ""
	if len(head) > 2 {
		payload = head[2]
	}

	// Messages whose payload may itself contain '|' need bounded splitting.
	switch typ {
	case "challstr":
		return ChallStr{Base: b, Challstr: payload}, true
	case "formats":
		return FormatsUpdated{Base: b, Formats: parseFormats(payload)}, true
	case "request":
		return BattleRequest{Base: b, Request: payload}, true
	case "updatesearch":
		return parseSearch(b, payload), true
	case "updatechallenges":
		return parseChallenges(b, payload), true
	case "queryresponse":
		q := splitN(line, 4)
		if len(q) < 4 {
			return nil, false
		}
		return QueryResponse{Base: b, Type: q[2], Data: q[3]}, true
	case "updateuser":
		u := splitN(line, 6)
		ev := UpdateUser{Base: b}
		if len(u) > 2 {
			ev.Name = u[2]
		}
		if len(u) > 3 {
			ev.Named = u[3] == "1"
		}
		if len(u) > 4 {
			ev.Avatar = u[4]
		}
		if len(u) > 5 {
			ev.Settings = u[5]
		}
		return ev, true
	case "c":
		p := splitN(line, 4)
		ev := ChatMessage{Base: b}
		if len(p) > 2 {
			ev.User = p[2]
		}
		if len(p) > 3 {
			ev.Message = p[3]
		}
		return ev, true
	case "c:":
		p := splitN(line, 5)
		ev := ChatMessage{Base: b}
		if len(p) > 2 {
			ev.Time = atoi64(p[2])
		}
		if len(p) > 3 {
			ev.User = p[3]
		}
		if len(p) > 4 {
			ev.Message = p[4]
		}
		return ev, true
	case "chat":
		p := splitN(line, 4)
		ev := ChatMessage{Base: b}
		if len(p) > 2 {
			ev.User = p[2]
		}
		if len(p) > 3 {
			ev.Message = p[3]
		}
		return ev, true
	case "pm":
		p := splitN(line, 5)
		ev := PM{Base: b}
		if len(p) > 2 {
			ev.Sender = p[2]
		}
		if len(p) > 3 {
			ev.Receiver = p[3]
		}
		if len(p) > 4 {
			ev.Message = p[4]
		}
		return ev, true
	case "battle", "b":
		p := splitN(line, 5)
		ev := BattleStarted{Base: b}
		if len(p) > 2 {
			ev.Base.RoomID = p[2]
		}
		if len(p) > 3 {
			ev.P1 = p[3]
		}
		if len(p) > 4 {
			ev.P2 = p[4]
		}
		return ev, true
	case "popup":
		return Popup{Base: b, Message: payload}, true
	case "usercount":
		return UserCount{Base: b, Count: atoi(payload)}, true
	case "nametaken":
		p := splitN(line, 4)
		ev := NameTaken{Base: b}
		if len(p) > 2 {
			ev.Name = p[2]
		}
		if len(p) > 3 {
			ev.Message = p[3]
		}
		return ev, true
	case "notify":
		p := splitN(line, 5)
		ev := Notify{Base: b}
		if len(p) > 2 {
			ev.Title = p[2]
		}
		if len(p) > 3 {
			ev.Message = p[3]
		}
		if len(p) > 4 {
			ev.Highlight = p[4]
		}
		return ev, true
	case "title":
		return RoomTitle{Base: b, Title: payload}, true
	case "users":
		return RoomUsers{Base: b, Users: parseUsers(payload)}, true
	case "html":
		return RoomHTML{Base: b, HTML: payload}, true
	case "init":
		return RoomInit{Base: b, Type: payload}, true
	case "j", "join":
		return RoomJoin{Base: b, User: payload}, true
	case "l", "leave":
		return RoomLeave{Base: b, User: payload}, true
	case "n", "name":
		p := splitN(line, 4)
		ev := RoomName{Base: b}
		if len(p) > 2 {
			ev.User = p[2]
		}
		if len(p) > 3 {
			ev.OldID = p[3]
		}
		return ev, true
	case "J", "L", "N", "B":
		// Uppercase variants are the same messages, flagged as too frequent to
		// display inline. We parse them but the UI suppresses them.
		switch typ {
		case "J":
			return RoomJoin{Base: b, User: payload}, true
		case "L":
			return RoomLeave{Base: b, User: payload}, true
		case "N":
			p := splitN(line, 4)
			ev := RoomName{Base: b}
			if len(p) > 2 {
				ev.User = p[2]
			}
			if len(p) > 3 {
				ev.OldID = p[3]
			}
			return ev, true
		}
		return nil, false
	case "":
		return RoomMessage{Base: b, Message: payload}, true
	}

	return parseBattle(b, typ, line)
}

// parseBattle handles the simulator's message types.
func parseBattle(b Base, typ, line string) (Event, bool) {
	args, tags := splitArgs(line)
	arg := func(i int) string {
		if i < len(args) {
			return args[i]
		}
		return ""
	}

	switch typ {
	// ---- battle initialisation ----
	case "player":
		return BattlePlayer{Base: b, Player: arg(1), Name: arg(2), Avatar: arg(3), Rating: arg(4)}, true
	case "teamsize":
		return BattleTeamSize{Base: b, Player: arg(1), Size: atoi(arg(2))}, true
	case "gametype":
		return BattleGameType{Base: b, GameType: arg(1)}, true
	case "gen":
		return BattleGen{Base: b, Gen: atoi(arg(1))}, true
	case "tier":
		return BattleTier{Base: b, Tier: arg(1)}, true
	case "rated":
		return BattleRated{Base: b, Message: arg(1)}, true
	case "rule":
		return BattleRule{Base: b, Rule: arg(1)}, true
	case "clearpoke":
		return BattleClearpoke{Base: b}, true
	case "poke":
		return BattlePoke{Base: b, Player: arg(1), Details: arg(2), Item: arg(3)}, true
	case "teampreview":
		return BattleTeamPreview{Base: b}, true
	case "start":
		return BattleStart{Base: b}, true

	// ---- battle progress ----
	case "turn":
		return BattleTurn{Base: b, Turn: atoi(arg(1))}, true
	case "win":
		return BattleWin{Base: b, User: arg(1)}, true
	case "tie":
		return BattleTie{Base: b}, true
	case "inactive":
		return BattleInactive{Base: b, Message: arg(1), On: true}, true
	case "inactiveoff":
		return BattleInactive{Base: b, Message: arg(1), On: false}, true
	case "upkeep":
		return BattleUpkeep{Base: b}, true
	case "t:":
		return BattleTimestamp{Base: b, TS: atoi64(arg(1))}, true
	case "error":
		return BattleError{Base: b, Message: arg(1)}, true
	case "message", "-message":
		return BattleMessage{Base: b, Message: arg(1)}, true

	// ---- major actions ----
	case "move":
		return BattleMove{Base: b, User: arg(1), Move: arg(2), Target: arg(3), Tags: tags}, true
	case "switch":
		hp, st := splitHP(arg(3))
		return BattleSwitch{Base: b, User: arg(1), Details: arg(2), HP: hp, Status: st, Tags: tags}, true
	case "drag":
		hp, st := splitHP(arg(3))
		return BattleSwitch{Base: b, User: arg(1), Details: arg(2), HP: hp, Status: st, Drag: true, Tags: tags}, true
	case "detailschange":
		hp, st := splitHP(arg(3))
		return BattleDetailsChange{Base: b, User: arg(1), Details: arg(2), HP: hp, Status: st}, true
	case "-formechange":
		hp, st := splitHP(arg(3))
		return BattleFormeChange{Base: b, User: arg(1), Species: arg(2), HP: hp, Status: st}, true
	case "replace":
		hp, st := splitHP(arg(3))
		return BattleReplace{Base: b, User: arg(1), Details: arg(2), HP: hp, Status: st}, true
	case "swap":
		return BattleSwap{Base: b, User: arg(1), Position: atoi(arg(2))}, true
	case "cant":
		return BattleCant{Base: b, User: arg(1), Reason: arg(2), Move: arg(3)}, true
	case "faint":
		return BattleFaint{Base: b, User: arg(1)}, true

	// ---- damage / status ----
	case "-damage":
		hp, st := splitHP(arg(2))
		return BattleDamage{Base: b, Target: arg(1), HP: hp, Status: st, Tags: tags}, true
	case "-heal":
		hp, st := splitHP(arg(2))
		return BattleHeal{Base: b, Target: arg(1), HP: hp, Status: st, Tags: tags}, true
	case "-sethp":
		return BattleSetHP{Base: b, Target: arg(1), HP: arg(2)}, true
	case "-status":
		return BattleStatus{Base: b, Target: arg(1), Status: arg(2)}, true
	case "-curestatus":
		return BattleCureStatus{Base: b, Target: arg(1), Status: arg(2)}, true
	case "-cureteam":
		return BattleCureTeam{Base: b, User: arg(1)}, true

	// ---- stat changes ----
	case "-boost":
		return BattleBoost{Base: b, Target: arg(1), Stat: arg(2), Amount: atoi(arg(3)), Tags: tags}, true
	case "-unboost":
		return BattleBoost{Base: b, Target: arg(1), Stat: arg(2), Amount: -atoi(arg(3)), Tags: tags}, true
	case "-setboost":
		return BattleBoost{Base: b, Target: arg(1), Stat: arg(2), Amount: atoi(arg(3)), Set: true, Tags: tags}, true
	case "-swapboost":
		return BattleSwapBoost{Base: b, Source: arg(1), Target: arg(2), Stats: splitCSV(arg(3))}, true
	case "-invertboost":
		return BattleInvertBoost{Base: b, Target: arg(1)}, true
	case "-clearboost":
		return BattleClearBoost{Base: b, Target: arg(1)}, true
	case "-clearallboost":
		return BattleClearAllBoost{Base: b}, true
	case "-copyboost":
		return BattleCopyBoost{Base: b, Source: arg(1), Target: arg(2)}, true

	// ---- field ----
	case "-weather":
		return BattleWeather{Base: b, Weather: arg(1), Upkeep: tags.Has("upkeep")}, true
	case "-fieldstart":
		return BattleFieldStart{Base: b, Condition: arg(1)}, true
	case "-fieldend":
		return BattleFieldEnd{Base: b, Condition: arg(1)}, true
	case "-fieldactivate":
		return BattleFieldActivate{Base: b, Condition: arg(1)}, true
	case "-sidestart":
		return BattleSideStart{Base: b, Side: arg(1), Condition: arg(2)}, true
	case "-sideend":
		return BattleSideEnd{Base: b, Side: arg(1), Condition: arg(2)}, true
	case "-swapsideconditions":
		return BattleSwapSideConditions{Base: b}, true
	case "-start":
		return BattleVolatileStart{Base: b, Target: arg(1), Effect: arg(2)}, true
	case "-end":
		return BattleVolatileEnd{Base: b, Target: arg(1), Effect: arg(2)}, true

	// ---- items / abilities / formes ----
	case "-item":
		return BattleItem{Base: b, Target: arg(1), Item: arg(2), From: tags.Get("from")}, true
	case "-enditem":
		return BattleEndItem{Base: b, Target: arg(1), Item: arg(2), From: tags.Get("from"), Eat: tags.Has("eat")}, true
	case "-ability":
		return BattleAbility{Base: b, Target: arg(1), Ability: arg(2), From: tags.Get("from")}, true
	case "-endability":
		return BattleEndAbility{Base: b, Target: arg(1)}, true
	case "-transform":
		return BattleTransform{Base: b, Target: arg(1), Species: arg(2)}, true
	case "-mega":
		return BattleMega{Base: b, User: arg(1), Stone: arg(2)}, true
	case "-primal":
		return BattlePrimal{Base: b, User: arg(1)}, true
	case "-burst":
		return BattleBurst{Base: b, User: arg(1), Species: arg(2), Item: arg(3)}, true
	case "-zpower":
		return BattleZPower{Base: b, User: arg(1)}, true
	case "-zbroken":
		return BattleZBroken{Base: b, Target: arg(1)}, true
	case "-hint":
		return BattleHint{Base: b, Message: arg(1)}, true
	}

	// Anything else with a leading '-' is a minor action we do not model
	// individually; keep it verbatim so the log can still render it.
	if strings.HasPrefix(typ, "-") {
		return BattleEffect{Base: b, Kind: strings.TrimPrefix(typ, "-"), Args: args[1:], Tags: tags}, true
	}

	return BattleUnknown{Base: b, Type: typ, Args: args[1:], Tags: tags}, true
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func atoi64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// splitHP splits the protocol's combined "HP STATUS" field, for example
// "50/100 brn" or "62%" or "0 fnt".
func splitHP(s string) (hp, status string) {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func isAlnum(r byte) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
}

func parseUsers(s string) []User {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	users := make([]User, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		u := User{}
		// '@' is both a rank prefix and the status separator. Only the last
		// one, past the start, delimits a status message.
		if i := strings.LastIndexByte(p, '@'); i > 0 {
			u.Status = p[i+1:]
			p = p[:i]
		}
		if len(p) > 0 && !isAlnum(p[0]) {
			u.Rank = string(p[0])
			p = p[1:]
		}
		u.Name = p
		if u.Name == "" {
			continue
		}
		users = append(users, u)
	}
	return users
}

// parseFormats decodes the |formats| payload.
//
// The layout is a flat list of fields. A field that is empty or starts with a
// comma followed by a number is a section marker: the NEXT field is the section
// name. Any other field is a format entry, which is a display name optionally
// followed by a comma and a hex flag bitmask.
//
// This mirrors the reference client's decoder exactly, including the flag bits:
//
//	1   preset team (Random Battle and friends)
//	2   searchable on the ladder
//	4   challengeable
//	8   usable in tournaments
//	16  teambuilder level 50
//	32  partner format
//	64  best-of default
//	128 tera preview default
//	256 item clause default
//
// The older documented ",#" / ",," / "," suffix form is still accepted for
// backwards compatibility.
func parseFormats(list string) []Format {
	fields := strings.Split(list, "|")
	var out []Format
	section := ""
	expectSection := false

	for _, f := range fields {
		switch {
		case expectSection:
			section = f
			expectSection = false
		case f == ",LL":
			// Local ladder marker; carries no format.
		case isSectionMarker(f):
			expectSection = true
		default:
			out = append(out, parseFormatEntry(f, section))
		}
	}
	return out
}

// isSectionMarker reports whether a field introduces a section.
func isSectionMarker(s string) bool {
	if s == "" {
		return true
	}
	if s[0] != ',' {
		return false
	}
	rest := s[1:]
	if rest == "" {
		return true
	}
	_, err := strconv.ParseFloat(rest, 64)
	return err == nil
}

func parseFormatEntry(s, section string) Format {
	f := Format{Section: section}
	name := s

	if idx := strings.LastIndexByte(s, ','); idx >= 0 {
		if code, err := strconv.ParseInt(s[idx+1:], 16, 32); err == nil {
			name = s[:idx]
			f.Random = code&1 != 0
			f.Searchable = code&2 != 0
			f.Challengeable = code&4 != 0
			f.Tournament = code&8 != 0
			return finalizeFormat(f, name)
		}
	}

	switch {
	case strings.HasSuffix(name, ",#"):
		f.Random, f.Searchable, f.Challengeable = true, true, true
		name = strings.TrimSuffix(name, ",#")
	case strings.HasSuffix(name, ",,"):
		f.Searchable = true
		name = strings.TrimSuffix(name, ",,")
	case strings.HasSuffix(name, ","):
		f.Challengeable = true
		name = strings.TrimSuffix(name, ",")
	default:
		f.Searchable, f.Challengeable = true, true
	}
	return finalizeFormat(f, name)
}

func finalizeFormat(f Format, name string) Format {
	f.Name = name
	f.ID = ToID(name)
	return f
}
