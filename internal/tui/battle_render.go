package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/unnipv/pokemon-slowdown/internal/battle"
	"github.com/unnipv/pokemon-slowdown/internal/config"
)

// This file renders the battle screen. The design goal is playability: the
// moves, both active Pokémon, and what just happened are always on screen, and
// colour is used to identify information (types, status, stat changes, PP)
// rather than for decoration.

func (bv *battleView) render(width, height int, layout LayoutMode) string {
	bv.curLayout = layout
	s := bv.state()
	t := bv.theme

	header := bv.renderHeader(width, layout)

	var field, conds, moves []string
	if s.TeamPreview {
		field = bv.renderPreview(width, layout)
	} else {
		field = bv.renderField(width, layout)
	}
	if layout.ShowFieldConditions() {
		if cond := bv.renderConditions(); cond != "" {
			conds = append(conds, cond)
		}
	}
	if s.AwaitingChoice && !s.Ended && !s.TeamPreview {
		moves = bv.renderMoves(width, layout)
	} else if !s.Ended && !s.TeamPreview {
		moves = []string{"", t.Muted.Render(bv.waitingLine())}
	}

	// Give the log whatever vertical space is left after the essentials, so a
	// long history can never push the move list off the bottom of the screen.
	used := 1 + len(field) + len(conds) + len(moves)
	budget := height - used - 1
	if budget > maxLogTail(layout) {
		budget = maxLogTail(layout)
	}
	logTail := bv.renderLogTail(budget)

	lines := make([]string, 0, height)
	lines = append(lines, header)
	lines = append(lines, field...)
	lines = append(lines, conds...)
	lines = append(lines, logTail...)
	lines = append(lines, moves...)
	if s.Ended {
		lines = append(lines, bv.renderResult(width)...)
	}

	body := strings.Join(lines, "\n")
	if bv.overlay != overlayNone {
		body = bv.renderOverlay(body, width, height, layout)
	}
	return body
}

// maxLogTail is how many recent log lines to show inline, per layout.
func maxLogTail(layout LayoutMode) int {
	switch layout {
	case LayoutCinematic, LayoutStandard:
		return 3
	case LayoutSidecar:
		return 3
	default:
		return 2
	}
}

func (bv *battleView) renderHeader(width int, layout LayoutMode) string {
	s := bv.state()
	t := bv.theme

	right := ""
	if s.Turn > 0 {
		right = fmt.Sprintf("Turn %d", s.Turn)
	}
	if s.TimerOn {
		right += " ⏱ timer"
	}
	rightW := lipgloss.Width(right)

	title := " slowdown"
	if layout == LayoutCompact {
		title = " s/d"
	}
	head := t.Title.Render(title) + t.Muted.Render(" · "+SanitizeLine(bv.title()))
	budget := width - rightW - 2
	if budget < lipgloss.Width(t.Title.Render(title)) {
		// Not even room for the title plus the turn counter.
		head = t.Title.Render(title)
	} else if lipgloss.Width(head) > budget {
		// Drop the format name, keeping the title and the turn counter.
		head = t.Title.Render(title)
	}

	pad := width - lipgloss.Width(head) - rightW
	if pad < 1 {
		pad = 1
	}
	return head + strings.Repeat(" ", pad) + t.Fg.Render(right)
}

// ---------------------------------------------------------------------------
// Field
// ---------------------------------------------------------------------------

func (bv *battleView) renderField(width int, layout LayoutMode) []string {
	s := bv.state()
	var out []string
	out = append(out, "")

	if layout.SpriteLayout() {
		out = append(out, bv.renderMonBlock(s.Opponent(), true, width, layout)...)
		out = append(out, "")
		out = append(out, bv.renderMonBlock(s.MySide(), false, width, layout)...)
		out = append(out, "")
		return out
	}

	// Narrow layouts: one line per active Pokémon, opponent first.
	for _, p := range activeOf(s.Opponent()) {
		out = append(out, bv.renderMonLine(p, true, width, layout))
	}
	for _, p := range activeOf(s.MySide()) {
		out = append(out, bv.renderMonLine(p, false, width, layout))
	}
	out = append(out, "")
	return out
}

func activeOf(side *battle.Side) []*battle.Pokemon {
	if side == nil {
		return nil
	}
	return side.ActiveParty()
}

// renderMonBlock is the wide layout: a sprite beside two lines of detail.
func (bv *battleView) renderMonBlock(side *battle.Side, foe bool, width int, layout LayoutMode) []string {
	if side == nil {
		return nil
	}
	t := bv.theme
	var out []string

	label := side.Name
	if foe {
		label = "opponent · " + side.Name
	}
	out = append(out, t.Muted.Render("  "+SanitizeLine(label)))

	active := side.ActiveParty()
	if len(active) == 0 {
		out = append(out, t.Dim.Render("  (no active Pokémon)"))
		return out
	}
	for _, p := range active {
		sprite := bv.spriteBlock(p, layout)
		info := bv.renderPokemonInfo(p, width-24, foe, layout)
		out = append(out, lipgloss.JoinHorizontal(lipgloss.Top, "  ", sprite, "  ", info))
	}
	return out
}

// renderPokemonInfo is the two-line detail block next to a sprite.
func (bv *battleView) renderPokemonInfo(p *battle.Pokemon, width int, foe bool, layout LayoutMode) string {
	t := bv.theme
	if p == nil {
		return ""
	}

	name := SanitizeLine(p.Name)
	if g := genderGlyph(p.Gender); g != "" {
		name += " " + g
	}
	title := t.Fg.Bold(true).Render(name)
	if p.Fainted {
		title = t.Dim.Render(name + "  fainted")
	}
	if types := bv.typeList(p.Species, layout); types != "" {
		title += "  " + types
	}

	line2 := bv.hpBar(p.HPPercent, 14) + " " + bv.hpText(p, foe)
	if st := bv.statusBadge(p.Status); st != "" {
		line2 += "  " + st
	}
	if boosts := bv.boostParts(p); len(boosts) > 0 {
		line2 += "  " + strings.Join(boosts, " ")
	}
	if p.Terastallized {
		line2 += "  " + t.Primary.Render("tera:"+p.TeraType)
	}
	if !foe && p.Item != "" {
		line2 += "  " + t.Muted.Render(p.Item)
	}
	return title + "\n" + line2
}

// renderMonLine is the narrow layout: everything important on one line.
func (bv *battleView) renderMonLine(p *battle.Pokemon, foe bool, width int, layout LayoutMode) string {
	t := bv.theme
	if p == nil {
		return ""
	}

	label := t.Muted.Render("you")
	if foe {
		label = t.Muted.Render("foe")
	}

	rawName := SanitizeLine(p.Name)
	if g := genderGlyph(p.Gender); g != "" {
		rawName += " " + g
	}

	hpText := bv.hpText(p, foe)
	types := bv.typeList(p.Species, layout)
	status := bv.statusBadge(p.Status)
	boosts := strings.Join(bv.boostParts(p), " ")

	// Compact keeps the types by shortening the name instead of dropping
	// information. Types are what tell you what you are actually looking at.
	displayName := rawName
	if layout == LayoutCompact {
		reserved := lipgloss.Width(label) + 1 + lipgloss.Width(hpText) + 2
		if types != "" {
			reserved += lipgloss.Width(types) + 1
		}
		if status != "" {
			reserved += lipgloss.Width(status) + 1
		}
		avail := width - reserved
		if avail < 8 {
			avail = 8
		}
		displayName = truncate(rawName, avail)
	}

	nameText := t.Fg.Bold(true).Render(displayName)
	if p.Fainted {
		nameText = t.Dim.Render(displayName + " fainted")
	}

	barW := 8
	if layout == LayoutCinematic {
		barW = 12
	}
	core := []string{label, nameText}
	// Compact drops the bar rather than the types: a number is enough to read
	// HP, and knowing what you are looking at matters more.
	if layout != LayoutCompact {
		core = append(core, bv.hpBar(p.HPPercent, barW))
	}
	core = append(core, hpText)

	// Extra information, least important last so it is dropped first when the
	// line does not fit.
	var extra []string
	if status != "" {
		extra = append(extra, status)
	}
	if boosts != "" {
		extra = append(extra, boosts)
	}
	if types != "" {
		extra = append(extra, types)
	}

	join := func(parts []string) string { return strings.Join(parts, " ") }
	line := join(append(append([]string{}, core...), extra...))
	// The line is left-aligned, so it may use the full width before dropping
	// anything.
	for lipgloss.Width(line) > width && len(extra) > 0 {
		extra = extra[:len(extra)-1]
		line = join(append(append([]string{}, core...), extra...))
	}
	return line
}

func (bv *battleView) hpText(p *battle.Pokemon, foe bool) string {
	if !foe && p.MaxHP > 0 {
		return bv.theme.Muted.Render(fmt.Sprintf("%d/%d", p.HP, p.MaxHP))
	}
	return bv.theme.Muted.Render(fmt.Sprintf("%d%%", p.HPPercent))
}

func (bv *battleView) hpBar(pct, width int) string {
	pct = clamp(pct, 0, 100)
	filled := pct * width / 100
	if pct > 0 && filled == 0 {
		filled = 1
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	style := bv.theme.HPFull
	switch {
	case pct <= 20:
		style = bv.theme.HPLow
	case pct <= 50:
		style = bv.theme.HPHalf
	}
	return style.Render(bar)
}

func (bv *battleView) statusBadge(status string) string {
	if status == "" || status == "fnt" {
		return ""
	}
	st, ok := bv.theme.Statuses[status]
	if !ok {
		st = bv.theme.Danger
	}
	return st.Render(strings.ToUpper(status))
}

var statAbbrev = map[string]string{
	"atk": "Atk", "def": "Def", "spa": "SpA", "spd": "SpD", "spe": "Spe",
	"accuracy": "Acc", "evasion": "Eva",
}

// boostParts renders each stat change, coloured by direction so a glance tells
// you whether a Pokémon is up or down.
func (bv *battleView) boostParts(p *battle.Pokemon) []string {
	if p == nil {
		return nil
	}
	var out []string
	for _, stat := range []string{"atk", "def", "spa", "spd", "spe"} {
		n := p.Boosts[stat]
		if n == 0 {
			continue
		}
		label := statAbbrev[stat]
		if label == "" {
			label = stat
		}
		text := fmt.Sprintf("%s%+d", label, n)
		if n > 0 {
			out = append(out, bv.theme.Success.Render(text))
		} else {
			out = append(out, bv.theme.Danger.Render(text))
		}
	}
	return out
}

func (bv *battleView) boostText(p *battle.Pokemon) string {
	parts := bv.boostParts(p)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// typeList renders a Pokémon's types as coloured badges.
func (bv *battleView) typeList(species string, layout LayoutMode) string {
	if bv.deps.Dex == nil || species == "" {
		return ""
	}
	sp, ok := bv.deps.Dex.Species(species)
	if !ok {
		return ""
	}
	var parts []string
	for _, typ := range sp.Types {
		name := strings.ToUpper(typ)
		if layout == LayoutCompact && len(name) > 3 {
			name = name[:3]
		}
		if st, ok := bv.theme.Types[strings.ToLower(typ)]; ok {
			parts = append(parts, st.Render(name))
		} else {
			parts = append(parts, bv.theme.Muted.Render(name))
		}
	}
	return strings.Join(parts, "")
}

func (bv *battleView) renderConditions() string {
	s := bv.state()
	t := bv.theme
	var parts []string
	if s.Weather != "" {
		parts = append(parts, t.Accent.Render(weatherLabel(s.Weather)))
	}
	if s.Terrain != "" {
		parts = append(parts, t.Accent.Render(cleanLabel(s.Terrain)))
	}
	for cond := range s.Field {
		if cond == s.Terrain {
			continue
		}
		parts = append(parts, t.Muted.Render(cleanLabel(cond)))
	}
	for _, side := range []*battle.Side{s.P1, s.P2} {
		if side == nil || len(side.Conditions) == 0 {
			continue
		}
		label := side.Name
		if s.Me != "" && side.ID == s.Me {
			label = "you"
		}
		var conds []string
		for c := range side.Conditions {
			conds = append(conds, cleanLabel(c))
		}
		parts = append(parts, t.Muted.Render(label+": "+strings.Join(conds, " ")))
	}
	if len(parts) == 0 {
		return ""
	}
	return "  " + strings.Join(parts, "  ")
}

// renderLogTail shows the most recent battle events inline. Without it, HP
// silently drops and statuses appear with no explanation.
func (bv *battleView) renderLogTail(n int) []string {
	if n <= 0 {
		return nil
	}
	log := bv.state().Log
	end := len(log)
	for end > 0 && log[end-1].Text == "" {
		end--
	}
	start := end - n
	if start < 0 {
		start = 0
	}
	if start >= end {
		return nil
	}
	out := make([]string, 0, n)
	for _, e := range log[start:end] {
		out = append(out, "  "+bv.styleLogLine(e))
	}
	return out
}

// ---------------------------------------------------------------------------
// Moves
// ---------------------------------------------------------------------------

func (bv *battleView) renderMoves(width int, layout LayoutMode) []string {
	s := bv.state()
	t := bv.theme
	req := s.Request
	if req == nil {
		return nil
	}
	if req.Kind() == battle.RequestSwitch {
		return []string{"", t.Danger.Render("  You must switch in a Pokémon.  ") + t.Muted.Render("press s or 1-6")}
	}
	ar := req.ActiveAt(bv.slot)
	if ar == nil {
		return nil
	}

	nameW := 16
	switch layout {
	case LayoutCinematic:
		nameW = 18
	case LayoutSidecar:
		nameW = 12
	case LayoutCompact:
		nameW = 10
	}

	var out []string
	out = append(out, "")
	if len(req.Active) > 1 {
		out = append(out, t.Muted.Render(fmt.Sprintf("  choosing for slot %d of %d", bv.slot+1, len(req.Active))))
	}

	mech := battle.AvailableMechanics(ar)
	for i, mv := range ar.Moves {
		out = append(out, bv.renderMoveLine(i+1, mv, width, nameW, layout))
	}

	var hints []string
	if ch, ok := bv.draft[bv.slot]; ok {
		hints = append(hints, t.Success.Render("chosen: "+ch.String()))
	}
	if bv.pendingMechanic != "" {
		hints = append(hints, t.Primary.Render("mechanic: "+bv.pendingMechanic))
	}
	if m, ok := mech.Primary(); ok {
		hints = append(hints, t.Primary.Render("[t] "+m.Label))
	}
	if bv.canSwitch() {
		hints = append(hints, t.Muted.Render("[s] switch"))
	}
	if len(hints) > 0 {
		out = append(out, "  "+strings.Join(hints, "   "))
	}
	return out
}

func (bv *battleView) renderMoveLine(n int, mv battle.MoveRequest, width, nameW int, layout LayoutMode) string {
	t := bv.theme

	typ := ""
	if bv.deps.Dex != nil {
		if m, ok := bv.deps.Dex.Move(mv.ID); ok {
			typ = m.Type
		}
	}

	key := t.Accent.Bold(true).Render(fmt.Sprintf("%d", n))

	// Pad the name AFTER styling: lipgloss normalises whitespace inside a
	// styled string, so padding first would be silently trimmed and the PP
	// column would drift by a character per row.
	label := truncate(mv.Move, nameW)
	var styled string
	switch {
	case mv.Disabled.Set:
		styled = t.Disabled.Render(label)
	default:
		if st, ok := t.TypesFg[strings.ToLower(typ)]; ok {
			styled = st.Render(label)
		} else {
			styled = t.Fg.Render(label)
		}
	}
	name := padRight(styled, nameW)

	left := fmt.Sprintf("  %s %s", key, name)
	if layout != LayoutCompact {
		left += " " + bv.typeBadge(typ)
	}

	pp := bv.ppText(mv)
	pad := width - lipgloss.Width(left) - lipgloss.Width(pp) - 2
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + pp
}

// typeBadge renders a fixed-width badge so badges line up in a column. The
// style's own padding is disabled and the label is padded to a fixed width
// afterwards: lipgloss normalises whitespace inside a styled string, so
// "GHOST   " and "FIGHTING" would otherwise render at different widths and
// push the PP column out of alignment.
func (bv *battleView) typeBadge(typ string) string {
	const badgeWidth = 8
	if typ == "" {
		return strings.Repeat(" ", badgeWidth+2)
	}
	st, ok := bv.theme.Types[strings.ToLower(typ)]
	if !ok {
		st = bv.theme.Muted
	}
	inner := st.Padding(0).Render(" " + strings.ToUpper(typ) + " ")
	return padRight(inner, badgeWidth+2)
}

// ppText colours remaining PP so a low or empty move is obvious.
func (bv *battleView) ppText(mv battle.MoveRequest) string {
	text := fmt.Sprintf("%d/%d", mv.PP, mv.MaxPP)
	switch {
	case mv.PP == 0:
		return bv.theme.Danger.Render(text)
	case mv.MaxPP > 0 && mv.PP*4 <= mv.MaxPP:
		return bv.theme.Warning.Render(text)
	default:
		return bv.theme.Muted.Render(text)
	}
}

// ---------------------------------------------------------------------------
// Team preview and result
// ---------------------------------------------------------------------------

func (bv *battleView) renderPreview(width int, layout LayoutMode) []string {
	s := bv.state()
	t := bv.theme
	out := []string{
		"",
		t.Title.Render("  Choose your lead order"),
		t.Muted.Render("  press a number to bring a Pokémon forward, enter to confirm"),
		"",
	}
	side := s.MySide()
	if side == nil {
		return out
	}
	picks := map[int]int{}
	for i, v := range bv.previewOrder {
		picks[v] = i + 1
	}
	for i, p := range side.Party {
		slot := i + 1
		marker := t.Muted.Render(" ")
		if n, ok := picks[slot]; ok {
			marker = t.Accent.Bold(true).Render(fmt.Sprintf("%d", n))
		}
		line := fmt.Sprintf("  [%s] %s", marker, t.Fg.Bold(true).Render(SanitizeLine(p.Name)))
		if types := bv.typeList(p.Species, layout); types != "" {
			line += "  " + types
		}
		out = append(out, line)
	}
	out = append(out, "")
	return out
}

func (bv *battleView) renderResult(width int) []string {
	s := bv.state()
	t := bv.theme
	out := []string{""}
	switch {
	case s.Tie:
		out = append(out, t.Warning.Render("  The battle ended in a tie."))
	case s.Winner != "":
		won := bv.wonByMe(s)
		text := "  " + SanitizeLine(s.Winner) + " won the battle!"
		if won {
			out = append(out, t.Success.Bold(true).Render(text))
			out = append(out, t.Muted.Render("  "+bv.tagline(config.SlotVictory)))
		} else {
			out = append(out, t.Danger.Bold(true).Render(text))
			out = append(out, t.Muted.Render("  "+bv.tagline(config.SlotDefeat)))
		}
	}
	out = append(out, "")
	out = append(out, t.Muted.Render("  replay: https://replay.pokemonshowdown.com/"+s.RoomID))
	out = append(out, t.Dim.Render("  esc back to lobby · tab next battle"))
	return out
}

// ---------------------------------------------------------------------------
// Sprites
// ---------------------------------------------------------------------------

// spriteBlock returns the rendered sprite for a Pokémon, or a quiet placeholder
// while it loads. Sprites never block gameplay: the box is always reserved.
func (bv *battleView) spriteBlock(p *battle.Pokemon, layout LayoutMode) string {
	cols, rows := layout.SpriteCells()
	if cols == 0 || bv.deps.Renderer == nil || p == nil || p.Species == "" {
		return ""
	}
	animate := bv.owner != nil && bv.owner.cfg.Sprites.Animate
	ref := bv.spriteRef(p, bv.isMine(p), animate)
	if s, ok := bv.spriteRendered[ref.Key()]; ok && s != "" {
		return s
	}
	return bv.placeholderBlock(cols, rows)
}

func (bv *battleView) placeholderBlock(cols, rows int) string {
	if cols <= 0 || rows <= 0 {
		return ""
	}
	line := bv.theme.Dim.Render(strings.Repeat("·", cols))
	lines := make([]string, rows)
	for i := range lines {
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func (bv *battleView) waitingLine() string {
	s := bv.state()
	if s.Ended {
		return ""
	}
	tag := bv.tagline(config.SlotWaiting)
	label := "◌ waiting"
	if s.P1.Name != "" || s.P2.Name != "" {
		label = "◌ " + s.P1.Name + " vs " + s.P2.Name + " · waiting"
	}
	if tag != "" {
		label += "  " + tag
	}
	return label
}

func genderGlyph(g string) string {
	switch g {
	case "M":
		return "♂"
	case "F":
		return "♀"
	}
	return ""
}

func weatherLabel(w string) string {
	switch cleanLabel(w) {
	case "RainDance":
		return "rain"
	case "Sandstorm":
		return "sandstorm"
	case "SunnyDay":
		return "sun"
	case "Hail":
		return "hail"
	case "Snow":
		return "snow"
	case "DesolateLand":
		return "harsh sunlight"
	case "PrimordialSea":
		return "heavy rain"
	case "DeltaStream":
		return "strong winds"
	}
	return cleanLabel(w)
}

func cleanLabel(s string) string {
	for _, p := range []string{"move: ", "ability: ", "item: "} {
		s = strings.TrimPrefix(s, p)
	}
	return s
}

// padRight pads s with spaces to width n, using rune width.
func padRight(s string, n int) string {
	w := lipgloss.Width(s)
	if w >= n {
		return s
	}
	return s + strings.Repeat(" ", n-w)
}
