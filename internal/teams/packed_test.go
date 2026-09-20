package teams

import (
	"path/filepath"
	"strings"
	"testing"
)

const sampleExport = `Gengar (Spooky) @ Choice Specs
Ability: Cursed Body
Tera Type: Ghost
EVs: 252 SpA / 4 SpD / 252 Spe
Timid Nature
IVs: 0 Atk
- Shadow Ball
- Sludge Wave
- Focus Blast
- Nasty Plot

Landorus-Therian @ Leftovers
Ability: Intimidate
Tera Type: Ground
EVs: 248 HP / 8 Def / 252 Spe
Jolly Nature
- Earthquake
- U-turn
- Stealth Rock
- Stone Edge
`

func TestParseExport(t *testing.T) {
	team, err := ParseExport(sampleExport)
	if err != nil {
		t.Fatalf("ParseExport: %v", err)
	}
	if len(team.Pokemon) != 2 {
		t.Fatalf("got %d Pokémon, want 2", len(team.Pokemon))
	}

	g := team.Pokemon[0]
	if g.Species != "Gengar" || g.Name != "Spooky" {
		t.Errorf("name/species = %q/%q", g.Name, g.Species)
	}
	if g.Item != "Choice Specs" || g.Ability != "Cursed Body" {
		t.Errorf("item/ability = %q/%q", g.Item, g.Ability)
	}
	if g.TeraType != "Ghost" {
		t.Errorf("tera type = %q", g.TeraType)
	}
	if g.Nature != "Timid" {
		t.Errorf("nature = %q", g.Nature)
	}
	if g.EVs.SpA != 252 || g.EVs.SpD != 4 || g.EVs.Spe != 252 || g.EVs.HP != 0 {
		t.Errorf("EVs = %+v", g.EVs)
	}
	if g.IVs.Atk != 0 {
		t.Errorf("IVs = %+v", g.IVs)
	}
	if len(g.Moves) != 4 || g.Moves[0] != "Shadow Ball" {
		t.Errorf("moves = %#v", g.Moves)
	}
	if g.Level != 100 {
		t.Errorf("default level = %d, want 100", g.Level)
	}

	l := team.Pokemon[1]
	if l.Species != "Landorus-Therian" || l.Name != "" {
		t.Errorf("second species/name = %q/%q", l.Species, l.Name)
	}
	if l.EVs.HP != 248 || l.EVs.Def != 8 {
		t.Errorf("second EVs = %+v", l.EVs)
	}
}

func TestExportRoundTrips(t *testing.T) {
	team, err := ParseExport(sampleExport)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ParseExport(team.Export())
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if len(again.Pokemon) != len(team.Pokemon) {
		t.Fatalf("round trip changed team size")
	}
	for i := range team.Pokemon {
		a, b := team.Pokemon[i], again.Pokemon[i]
		if a.Species != b.Species || a.Item != b.Item || a.Ability != b.Ability ||
			a.Nature != b.Nature || a.TeraType != b.TeraType || a.EVs != b.EVs {
			t.Errorf("round trip mismatch at %d:\n%+v\n%+v", i, a, b)
		}
		if strings.Join(a.Moves, ",") != strings.Join(b.Moves, ",") {
			t.Errorf("moves mismatch at %d: %#v vs %#v", i, a.Moves, b.Moves)
		}
	}
}

func TestPackedRoundTrip(t *testing.T) {
	team, err := ParseExport(sampleExport)
	if err != nil {
		t.Fatal(err)
	}
	packed := team.Pack()
	if !strings.Contains(packed, "]") {
		t.Fatalf("packed teams should separate Pokémon with ']': %q", packed)
	}
	if !strings.HasPrefix(packed, "Spooky|Gengar|Choice Specs|Cursed Body|") {
		t.Errorf("packed head wrong: %q", packed)
	}

	back, err := Unpack(packed)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if len(back.Pokemon) != 2 {
		t.Fatalf("unpacked %d Pokémon", len(back.Pokemon))
	}
	g := back.Pokemon[0]
	if g.Species != "Gengar" || g.Name != "Spooky" || g.Nature != "Timid" {
		t.Errorf("unpacked head wrong: %+v", g)
	}
	if g.EVs.SpA != 252 || g.IVs.Atk != 0 {
		t.Errorf("unpacked stats wrong: EVs=%+v IVs=%+v", g.EVs, g.IVs)
	}
	if len(g.Moves) != 4 || g.Moves[1] != "Sludge Wave" {
		t.Errorf("unpacked moves wrong: %#v", g.Moves)
	}
}

func TestUnpackNullAndEmpty(t *testing.T) {
	for _, in := range []string{"", "null", "  "} {
		team, err := Unpack(in)
		if err != nil {
			t.Errorf("Unpack(%q) errored: %v", in, err)
		}
		if len(team.Pokemon) != 0 {
			t.Errorf("Unpack(%q) = %d Pokémon, want 0", in, len(team.Pokemon))
		}
		if got := team.PackedOrNull(); got != "null" {
			t.Errorf("PackedOrNull(%q) = %q", in, got)
		}
	}
}

func TestUnpackLevelAndShiny(t *testing.T) {
	team, err := Unpack("Pikachu|Pikachu|Light Ball|Static|thunderbolt,quickattack|||F||S|50")
	if err != nil {
		t.Fatal(err)
	}
	p := team.Pokemon[0]
	if p.Level != 50 {
		t.Errorf("level = %d, want 50", p.Level)
	}
	if !p.Shiny {
		t.Error("shiny flag lost")
	}
	if p.Gender != "F" {
		t.Errorf("gender = %q, want F", p.Gender)
	}
}

func TestStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "teams.json")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.All()) != 0 {
		t.Fatal("new store should be empty")
	}

	team, _ := ParseExport(sampleExport)
	team.Name = "Rain Dance"
	if err := s.Add(team); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// Reload from disk to prove persistence.
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.All()) != 1 || s2.All()[0].Name != "Rain Dance" {
		t.Fatalf("persisted store wrong: %#v", s2.All())
	}

	if err := s2.Duplicate(0); err != nil {
		t.Fatalf("Duplicate: %v", err)
	}
	if len(s2.All()) != 2 || s2.All()[1].Name != "Rain Dance (copy)" {
		t.Errorf("duplicate name wrong: %#v", s2.All())
	}

	if err := s2.Delete(0); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(s2.All()) != 1 {
		t.Errorf("after delete: %d teams", len(s2.All()))
	}

	if i, ok := s2.Find("rain dance (copy)"); !ok || i != 0 {
		t.Errorf("Find returned %d, %v", i, ok)
	}
}

func TestParseExportSkipsBlankLinesAndUnknownDirectives(t *testing.T) {
	text := "\n\nGengar @ Leftovers\nAbility: Cursed Body\nHappiness: 255\n- Shadow Ball\n\n\n"
	team, err := ParseExport(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(team.Pokemon) != 1 || len(team.Pokemon[0].Moves) != 1 {
		t.Fatalf("unexpected team: %#v", team)
	}
}

func TestParseExportRejectsEmptySpecies(t *testing.T) {
	if _, err := ParseExport("Ability: Cursed Body\n- Shadow Ball\n"); err == nil {
		t.Fatal("expected an error for a block without a species")
	}
}
