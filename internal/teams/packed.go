package teams

import (
	"fmt"
	"strconv"
	"strings"
)

// Pack renders the team in Showdown's packed format, which is what the server
// expects for /utm, /search and /challenge.
func (t Team) Pack() string {
	parts := make([]string, 0, len(t.Pokemon))
	for _, p := range t.Pokemon {
		parts = append(parts, packOne(p))
	}
	return strings.Join(parts, "]")
}

// PackedOrNull returns "null" for formats that do not use user-built teams,
// such as Random Battle.
func (t Team) PackedOrNull() string {
	if len(t.Pokemon) == 0 {
		return "null"
	}
	return t.Pack()
}

func packOne(p Pokemon) string {
	name := p.Name
	if name == "" {
		name = p.Species
	}
	shiny := ""
	if p.Shiny {
		shiny = "S"
	}
	level := ""
	if p.Level > 0 && p.Level != 100 {
		level = strconv.Itoa(p.Level)
	}
	fields := []string{
		name,
		p.Species,
		p.Item,
		p.Ability,
		strings.Join(p.Moves, ","),
		p.Nature,
		statString(p.EVs),
		p.Gender,
		statString(p.IVs),
		shiny,
		level,
	}
	return strings.Join(fields, "|")
}

// statString renders six values, or "" when every value is zero. Empty EVs mean
// zero EVs; empty IVs mean the default of 31.
func statString(s Stats) string {
	vals := []int{s.HP, s.Atk, s.Def, s.SpA, s.SpD, s.Spe}
	allZero := true
	for _, v := range vals {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return ""
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = strconv.Itoa(v)
	}
	return strings.Join(parts, ",")
}

// Unpack parses Showdown's packed team format. An empty string or "null"
// yields an empty team.
func Unpack(s string) (Team, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return Team{}, nil
	}
	var t Team
	for _, chunk := range strings.Split(s, "]") {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		f := strings.Split(chunk, "|")
		get := func(i int) string {
			if i < len(f) {
				return f[i]
			}
			return ""
		}
		p := Pokemon{
			Name:    get(0),
			Species: get(1),
			Item:    get(2),
			Ability: get(3),
			Nature:  get(5),
			Gender:  get(7),
			Shiny:   get(9) == "S",
			Level:   atoi(get(10)),
			EVs:     parsePackedStats(get(6)),
			IVs:     parsePackedStats(get(8)),
		}
		if mv := get(4); mv != "" {
			p.Moves = strings.Split(mv, ",")
		}
		if p.Level == 0 {
			p.Level = 100
		}
		if p.Species == "" {
			p.Species = p.Name
		}
		if p.Species == "" {
			return Team{}, fmt.Errorf("teams: packed entry %q has no species", chunk)
		}
		t.Pokemon = append(t.Pokemon, p)
	}
	return t, nil
}

func parsePackedStats(s string) Stats {
	if s == "" {
		return Stats{}
	}
	parts := strings.Split(s, ",")
	n := func(i int) int {
		if i < len(parts) {
			return atoi(parts[i])
		}
		return 0
	}
	return Stats{HP: n(0), Atk: n(1), Def: n(2), SpA: n(3), SpD: n(4), Spe: n(5)}
}

// Export renders the team in Showdown's human-readable format, suitable for
// editing in $EDITOR or pasting into the web teambuilder.
func (t Team) Export() string {
	var sb strings.Builder
	for i, p := range t.Pokemon {
		if i > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(exportOne(p))
	}
	return sb.String()
}

func exportOne(p Pokemon) string {
	var sb strings.Builder
	sb.WriteString(p.Species)
	if p.Name != "" && p.Name != p.Species {
		sb.WriteString(" (" + p.Name + ")")
	}
	if p.Item != "" {
		sb.WriteString(" @ " + p.Item)
	}
	sb.WriteByte('\n')

	if p.Ability != "" {
		sb.WriteString("Ability: " + p.Ability + "\n")
	}
	if p.TeraType != "" {
		sb.WriteString("Tera Type: " + p.TeraType + "\n")
	}
	if p.Level != 0 && p.Level != 100 {
		sb.WriteString("Level: " + strconv.Itoa(p.Level) + "\n")
	}
	if p.Shiny {
		sb.WriteString("Shiny: Yes\n")
	}
	if p.Gender != "" {
		sb.WriteString("Gender: " + p.Gender + "\n")
	}
	if line := statsLine("EVs", p.EVs); line != "" {
		sb.WriteString(line + "\n")
	}
	if p.Nature != "" {
		sb.WriteString(p.Nature + " Nature\n")
	}
	if line := statsLine("IVs", p.IVs); line != "" {
		sb.WriteString(line + "\n")
	}
	for _, m := range p.Moves {
		if m == "" {
			continue
		}
		sb.WriteString("- " + m + "\n")
	}
	return sb.String()
}

func statsLine(label string, s Stats) string {
	var parts []string
	add := func(n int, name string) {
		if n != 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, name))
		}
	}
	add(s.HP, "HP")
	add(s.Atk, "Atk")
	add(s.Def, "Def")
	add(s.SpA, "SpA")
	add(s.SpD, "SpD")
	add(s.Spe, "Spe")
	if len(parts) == 0 {
		return ""
	}
	return label + ": " + strings.Join(parts, " / ")
}

// ParseExport parses Showdown's human-readable team format.
func ParseExport(text string) (Team, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var t Team
	for _, block := range splitBlocks(text) {
		p, err := parseOne(block)
		if err != nil {
			return Team{}, err
		}
		t.Pokemon = append(t.Pokemon, p)
	}
	return t, nil
}

func splitBlocks(text string) []string {
	var blocks []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			blocks = append(blocks, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		cur = append(cur, strings.TrimRight(line, " \t"))
	}
	flush()
	return blocks
}

func parseOne(block string) (Pokemon, error) {
	lines := strings.Split(block, "\n")
	if len(lines) == 0 {
		return Pokemon{}, fmt.Errorf("teams: empty block")
	}
	p := Pokemon{Level: 100}
	if looksLikeDirective(lines[0]) {
		return Pokemon{}, fmt.Errorf("teams: block does not start with a species: %q", lines[0])
	}
	p.Name, p.Species, p.Item = parseNameLine(lines[0])
	if p.Species == "" {
		return Pokemon{}, fmt.Errorf("teams: block has no species: %q", lines[0])
	}

	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		switch {
		case line == "":
		case strings.HasPrefix(line, "- "):
			p.Moves = append(p.Moves, strings.TrimSpace(strings.TrimPrefix(line, "-")))
		case strings.HasPrefix(line, "Ability:"):
			p.Ability = strings.TrimSpace(strings.TrimPrefix(line, "Ability:"))
		case strings.HasPrefix(line, "Tera Type:"):
			p.TeraType = strings.TrimSpace(strings.TrimPrefix(line, "Tera Type:"))
		case strings.HasPrefix(line, "Level:"):
			p.Level = atoi(strings.TrimSpace(strings.TrimPrefix(line, "Level:")))
		case strings.HasPrefix(line, "Shiny:"):
			p.Shiny = strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line, "Shiny:")), "yes")
		case strings.HasPrefix(line, "Gender:"):
			p.Gender = strings.TrimSpace(strings.TrimPrefix(line, "Gender:"))
		case strings.HasPrefix(line, "EVs:"):
			p.EVs = parseStatsLine(strings.TrimPrefix(line, "EVs:"))
		case strings.HasPrefix(line, "IVs:"):
			p.IVs = parseStatsLine(strings.TrimPrefix(line, "IVs:"))
		case strings.HasSuffix(line, "Nature"):
			p.Nature = strings.TrimSpace(strings.TrimSuffix(line, "Nature"))
		default:
			// Unknown or unmodelled directive (Happiness:, Hidden Power: and
			// friends): keep parsing rather than reject the whole team.
		}
	}
	if p.Level == 0 {
		p.Level = 100
	}
	return p, nil
}

func parseNameLine(line string) (name, species, item string) {
	line = strings.TrimSpace(line)
	if i := strings.Index(line, " @ "); i >= 0 {
		item = strings.TrimSpace(line[i+3:])
		line = strings.TrimSpace(line[:i])
	}
	if i := strings.LastIndex(line, " ("); i >= 0 && strings.HasSuffix(line, ")") {
		// Showdown writes "Species (Nickname)".
		species = strings.TrimSpace(line[:i])
		name = strings.TrimSpace(line[i+2 : len(line)-1])
		return name, species, item
	}
	return "", strings.TrimSpace(line), item
}

// looksLikeDirective reports whether a line is a team-editor directive rather
// than a species line, which means the block is malformed.
func looksLikeDirective(line string) bool {
	for _, prefix := range []string{
		"Ability:", "EVs:", "IVs:", "Level:", "Shiny:", "Gender:",
		"Tera Type:", "Happiness:", "Hidden Power:", "- ",
	} {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func parseStatsLine(s string) Stats {
	var st Stats
	for _, part := range strings.Split(s, "/") {
		fields := strings.Fields(strings.TrimSpace(part))
		if len(fields) != 2 {
			continue
		}
		n := atoi(fields[0])
		switch fields[1] {
		case "HP":
			st.HP = n
		case "Atk":
			st.Atk = n
		case "Def":
			st.Def = n
		case "SpA":
			st.SpA = n
		case "SpD":
			st.SpD = n
		case "Spe":
			st.Spe = n
		}
	}
	return st
}

func atoi(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
