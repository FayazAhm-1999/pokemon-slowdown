package showdown

import (
	"reflect"
	"testing"
)

func TestSplitFrames(t *testing.T) {
	raw := "|challstr|2|abc\n>battle-gen9randombattle-1\n|player|p1|Alice|1|1500\n|turn|1\n\n>lobby\n|c|Bob|hi there"
	frames := SplitFrames(raw)
	if len(frames) != 3 {
		t.Fatalf("got %d frames, want 3: %#v", len(frames), frames)
	}
	if frames[0].RoomID != "" || frames[0].Lines[0] != "|challstr|2|abc" {
		t.Errorf("global frame wrong: %#v", frames[0])
	}
	if frames[1].RoomID != "battle-gen9randombattle-1" || len(frames[1].Lines) != 2 {
		t.Errorf("battle frame wrong: %#v", frames[1])
	}
	if frames[2].RoomID != "lobby" {
		t.Errorf("lobby frame wrong: %#v", frames[2])
	}
}

func TestSplitFramesEmpty(t *testing.T) {
	if got := SplitFrames("\n\n"); len(got) != 0 {
		t.Fatalf("want no frames, got %#v", got)
	}
}

func TestParseChatKeepsPipes(t *testing.T) {
	evs := Parse(Frame{RoomID: "lobby", Lines: []string{"|c|Bob|hi | there | friend"}})
	if len(evs) != 1 {
		t.Fatalf("want 1 event, got %d", len(evs))
	}
	c, ok := evs[0].(ChatMessage)
	if !ok {
		t.Fatalf("want ChatMessage, got %T", evs[0])
	}
	if c.User != "Bob" || c.Message != "hi | there | friend" {
		t.Errorf("bad chat: %#v", c)
	}
}

func TestParseChatTimestamp(t *testing.T) {
	evs := Parse(Frame{RoomID: "lobby", Lines: []string{"|c:|1700000000|Bob|hey"}})
	c := evs[0].(ChatMessage)
	if c.Time != 1700000000 || c.User != "Bob" || c.Message != "hey" {
		t.Errorf("bad chat: %#v", c)
	}
}

func TestParseChallstrWithPipes(t *testing.T) {
	evs := Parse(Frame{Lines: []string{"|challstr|2|2023-09-01|abcdef|1234"}})
	cs, ok := evs[0].(ChallStr)
	if !ok {
		t.Fatalf("want ChallStr, got %T", evs[0])
	}
	if cs.Challstr != "2|2023-09-01|abcdef|1234" {
		t.Errorf("bad challstr: %q", cs.Challstr)
	}
}

func TestParseMoveWithTags(t *testing.T) {
	evs := Parse(Frame{RoomID: "battle-1", Lines: []string{"|move|p1a: Dragapult|Draco Meteor|p2a: Garchomp|[miss]"}})
	mv, ok := evs[0].(BattleMove)
	if !ok {
		t.Fatalf("want BattleMove, got %T", evs[0])
	}
	if mv.User != "p1a: Dragapult" || mv.Move != "Draco Meteor" || mv.Target != "p2a: Garchomp" {
		t.Errorf("bad move: %#v", mv)
	}
	if !mv.Tags.Has("miss") {
		t.Errorf("expected miss tag: %#v", mv.Tags)
	}
}

func TestParseDamageWithFrom(t *testing.T) {
	evs := Parse(Frame{RoomID: "battle-1", Lines: []string{"|-damage|p1a: X|50/100 brn|[from] item: Life Orb"}})
	d, ok := evs[0].(BattleDamage)
	if !ok {
		t.Fatalf("want BattleDamage, got %T", evs[0])
	}
	if d.HP != "50/100" || d.Status != "brn" {
		t.Errorf("bad damage: %#v", d)
	}
	if d.Tags.Get("from") != "item: Life Orb" {
		t.Errorf("bad from tag: %#v", d.Tags)
	}
}

func TestParseBoost(t *testing.T) {
	evs := Parse(Frame{Lines: []string{"|-boost|p1a: X|atk|2"}})
	bl := evs[0].(BattleBoost)
	if bl.Stat != "atk" || bl.Amount != 2 || bl.Set {
		t.Errorf("bad boost: %#v", bl)
	}
	evs = Parse(Frame{Lines: []string{"|-unboost|p2a: Y|spe|1"}})
	bu := evs[0].(BattleBoost)
	if bu.Amount != -1 {
		t.Errorf("unboost should be negative: %#v", bu)
	}
}

func TestParseUnknownIsPreserved(t *testing.T) {
	evs := Parse(Frame{RoomID: "battle-1", Lines: []string{"|-somefutureserverthing|p1a: X|whatever"}})
	eff, ok := evs[0].(BattleEffect)
	if !ok {
		t.Fatalf("want BattleEffect, got %T", evs[0])
	}
	if eff.Kind != "somefutureserverthing" || len(eff.Args) != 2 {
		t.Errorf("bad effect: %#v", eff)
	}

	evs = Parse(Frame{Lines: []string{"|brandnewtype|a|b"}})
	unk, ok := evs[0].(BattleUnknown)
	if !ok {
		t.Fatalf("want BattleUnknown, got %T", evs[0])
	}
	if unk.Type != "brandnewtype" {
		t.Errorf("bad unknown: %#v", unk)
	}
}

func TestParseFormats(t *testing.T) {
	payload := ",1|S/V Singles|gen9randombattle,#|gen9ou,,|gen9customgame,||,2|Past Gens|gen8randombattle,#"
	got := parseFormats(payload)
	want := []Format{
		{ID: "gen9randombattle", Section: "S/V Singles", Random: true, Searchable: true, Challengeable: true},
		{ID: "gen9ou", Section: "S/V Singles", Searchable: true},
		{ID: "gen9customgame", Section: "S/V Singles", Challengeable: true},
		{ID: "gen8randombattle", Section: "Past Gens", Random: true, Searchable: true, Challengeable: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("formats mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestParseUsers(t *testing.T) {
	got := parseUsers("@Alice, Bob,~Carol@!away")
	want := []User{
		{Name: "Alice", Rank: "@"},
		{Name: "Bob"},
		{Name: "Carol", Rank: "~", Status: "!away"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("users mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestToID(t *testing.T) {
	cases := map[string]string{
		"[Gen 9] Random Battle": "gen9randombattle",
		"Kingambit":             "kingambit",
		"Landorus-Therian":      "landorustherian",
	}
	for in, want := range cases {
		if got := ToID(in); got != want {
			t.Errorf("ToID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseBattleHeader(t *testing.T) {
	lines := []string{
		"|player|p1|Anonycat|60|1200",
		"|player|p2|Anonybird|113|1300",
		"|teamsize|p1|4",
		"|gametype|doubles",
		"|gen|7",
		"|tier|[Gen 7] Doubles Ubers",
		"|clearpoke",
		"|poke|p1|Pikachu, L59, F|item",
		"|teampreview",
		"|start",
	}
	evs := Parse(Frame{RoomID: "battle-gen7doubles-1", Lines: lines})
	if len(evs) != len(lines) {
		t.Fatalf("got %d events, want %d", len(evs), len(lines))
	}
	p := evs[0].(BattlePlayer)
	if p.Player != "p1" || p.Name != "Anonycat" || p.Rating != "1200" {
		t.Errorf("bad player: %#v", p)
	}
	if g := evs[3].(BattleGameType); g.GameType != "doubles" {
		t.Errorf("bad gametype: %#v", g)
	}
	pk := evs[7].(BattlePoke)
	if pk.Details != "Pikachu, L59, F" || pk.Item != "item" {
		t.Errorf("bad poke: %#v", pk)
	}
	if _, ok := evs[9].(BattleStart); !ok {
		t.Errorf("want BattleStart, got %T", evs[9])
	}
}
