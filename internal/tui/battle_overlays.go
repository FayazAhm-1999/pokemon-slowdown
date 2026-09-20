package tui

import (
	"fmt"
	"strings"

	"github.com/unnipv/pokemon-slowdown/internal/battle"
)

// renderOverlay draws the active overlay over the battle body.
func (bv *battleView) renderOverlay(base string, width, height int, layout LayoutMode) string {
	var box string
	switch bv.overlay {
	case overlaySwitch:
		box = bv.renderSwitchOverlay(width)
	case overlayInspect:
		box = bv.renderInspectOverlay(width)
	case overlayLog:
		box = bv.renderLogOverlay(width, height)
	case overlayChat:
		box = bv.renderChatOverlay(width, height)
	case overlayTarget:
		box = bv.renderTargetOverlay(width)
	}
	if box == "" {
		return base
	}
	return overlayAt(base, box, height)
}

// overlayAt replaces lines of the base with the overlay, positioned near the
// top third but always clamped so the whole overlay is visible.
func overlayAt(base, box string, height int) string {
	boxLines := strings.Split(box, "\n")
	baseLines := strings.Split(base, "\n")
	out := make([]string, len(baseLines))
	copy(out, baseLines)

	maxStart := len(out) - len(boxLines)
	if maxStart < 0 {
		maxStart = 0
	}
	start := (height - len(boxLines)) / 3
	if start < 0 {
		start = 0
	}
	if start > maxStart {
		start = maxStart
	}
	for i, ln := range boxLines {
		idx := start + i
		if idx >= 0 && idx < len(out) {
			out[idx] = ln
		}
	}
	return strings.Join(out, "\n")
}

func (bv *battleView) overlayFrame(title string, width int, body []string) string {
	if width < 20 {
		width = 20
	}
	inner := width - 6
	t := bv.theme
	head := t.Title.Render(title)
	lines := []string{head, ""}
	lines = append(lines, body...)
	lines = append(lines, "", t.Dim.Render("esc close"))
	content := strings.Join(lines, "\n")
	return t.BoxFocus.Width(inner).Padding(0, 2).Render(content)
}

func (bv *battleView) renderSwitchOverlay(width int) string {
	t := bv.theme
	slots := bv.switchSlots()
	if len(slots) == 0 {
		return bv.overlayFrame("Switch", width, []string{t.Muted.Render("No party information yet.")})
	}
	var body []string
	for i, sl := range slots {
		cursor := "  "
		if i == bv.overlayCursor {
			cursor = t.Accent.Render("> ")
		}
		name := sl.Pokemon.Ident
		if _, _, n := parseIdentLoose(sl.Pokemon.Ident); n != "" {
			name = n
		}
		state := ""
		if sl.Fainted {
			state = t.Dim.Render("fainted")
		} else if sl.Active {
			state = t.Muted.Render("already out")
		} else {
			state = t.Muted.Render(sl.Pokemon.Condition)
		}
		line := fmt.Sprintf("%s[%d] %s  %s", cursor, sl.Index, t.Fg.Render(name), state)
		if !sl.Legal {
			line = t.Disabled.Render(fmt.Sprintf("  [%d] %s  %s", sl.Index, name, "unavailable"))
		}
		body = append(body, line)
	}
	return bv.overlayFrame("Switch", width, body)
}

func (bv *battleView) renderInspectOverlay(width int) string {
	t := bv.theme
	p := bv.inspectTarget()
	if p == nil {
		return bv.overlayFrame("Inspect", width, []string{t.Muted.Render("Nothing to inspect.")})
	}

	title := SanitizeLine(p.Name)
	if p.Gender != "" {
		title += " " + genderGlyph(p.Gender)
	}
	body := []string{
		t.Fg.Bold(true).Render(title) + "  " + bv.typeList(p.Species, bv.curLayout),
		"",
		fmt.Sprintf("HP      %s %d%%", bv.hpBar(p.HPPercent, 12), p.HPPercent),
		fmt.Sprintf("Status  %s", orDash(bv.statusBadge(p.Status))),
	}
	if p.MaxHP > 0 && p.Active {
		body = append(body, fmt.Sprintf("Actual  %d/%d", p.HP, p.MaxHP))
	}
	if p.Active {
		body = append(body, fmt.Sprintf("Level   %d", p.Level))
	}
	if b := bv.boostText(p); b != "" {
		body = append(body, fmt.Sprintf("Boosts  %s", t.Accent.Render(b)))
	}
	if p.Terastallized {
		body = append(body, fmt.Sprintf("Tera    %s", t.Primary.Render(p.TeraType)))
	}
	// Only information the server has actually revealed is shown for the
	// opponent. Hidden information is never inferred.
	if bv.isMine(p) {
		if p.Item != "" {
			body = append(body, fmt.Sprintf("Item    %s", p.Item))
		}
		if p.Ability != "" {
			body = append(body, fmt.Sprintf("Ability %s", p.Ability))
		}
		if len(p.Moves) > 0 {
			body = append(body, "", t.Muted.Render("Moves"))
			for _, mv := range p.Moves {
				pp := fmt.Sprintf("%d/%d", mv.PP, mv.MaxPP)
				line := fmt.Sprintf("  %-18s %s", mv.Name, pp)
				if mv.Disabled {
					line = t.Disabled.Render(line + "  disabled")
				}
				body = append(body, line)
			}
		}
	} else {
		body = append(body, "", t.Dim.Render("Only revealed information is shown."))
		if p.Item != "" {
			body = append(body, fmt.Sprintf("Item    %s", p.Item))
		}
		if p.Ability != "" {
			body = append(body, fmt.Sprintf("Ability %s", p.Ability))
		}
	}
	return bv.overlayFrame("Inspect", width, body)
}

func (bv *battleView) inspectTarget() *battle.Pokemon {
	s := bv.state()
	if s.Request != nil {
		if p := s.MySide().ActiveAt(bv.slot); p != nil {
			return p
		}
	}
	if p := s.MySide().ActiveAt(0); p != nil {
		return p
	}
	if p := s.Opponent().ActiveAt(0); p != nil {
		return p
	}
	return nil
}

func (bv *battleView) isMine(p *battle.Pokemon) bool {
	s := bv.state()
	return s.Me != "" && p.SideID == s.Me
}

func (bv *battleView) renderLogOverlay(width, height int) string {
	s := bv.state()
	t := bv.theme
	rows := height - 8
	if rows < 4 {
		rows = 4
	}
	end := clamp(bv.logScroll, 0, len(s.Log))
	start := end - rows
	if start < 0 {
		start = 0
	}
	var body []string
	for _, e := range s.Log[start:end] {
		body = append(body, bv.styleLogLine(e))
	}
	if len(body) == 0 {
		body = append(body, t.Muted.Render("(nothing yet)"))
	}
	body = append(body, "", t.Dim.Render("↑/↓ scroll"))
	return bv.overlayFrame("Battle log", width, body)
}

func (bv *battleView) styleLogLine(e battle.LogEntry) string {
	t := bv.theme
	text := SanitizeLine(e.Text)
	if text == "" {
		return ""
	}
	switch e.Kind {
	case "move":
		return t.Fg.Render(text)
	case "damage", "faint":
		return t.Danger.Render(text)
	case "heal":
		return t.Success.Render(text)
	case "status", "boost":
		return t.Warning.Render(text)
	case "crit", "effect", "tera", "zmove", "mega":
		return t.Accent.Render(text)
	case "weather", "field", "side":
		return t.Primary.Render(text)
	case "turn":
		return t.Title.Render(text)
	case "error":
		return t.Danger.Render(text)
	case "hint", "spacer":
		return t.Dim.Render(text)
	default:
		return t.Muted.Render(text)
	}
}

func (bv *battleView) renderChatOverlay(width, height int) string {
	t := bv.theme
	rows := height - 10
	if rows < 3 {
		rows = 3
	}
	start := len(bv.chat) - rows
	if start < 0 {
		start = 0
	}
	var body []string
	for _, c := range bv.chat[start:] {
		name := t.Muted.Render(SanitizeLine(c.user))
		if c.me {
			name = t.Primary.Render(SanitizeLine(c.user))
		}
		body = append(body, name+": "+t.Fg.Render(c.text))
	}
	if len(body) == 0 {
		body = append(body, t.Muted.Render("(no messages)"))
	}
	body = append(body, "", t.Accent.Render("› ")+t.Fg.Render(bv.chatInput)+t.Primary.Render("▌"))
	return bv.overlayFrame("Chat", width, body)
}

func (bv *battleView) renderTargetOverlay(width int) string {
	s := bv.state()
	t := bv.theme
	var body []string
	for i, opt := range bv.targets {
		cursor := "  "
		if i == bv.overlayCursor {
			cursor = t.Accent.Render("> ")
		}
		label := bv.targetLabel(opt)
		side := "ally"
		if opt.Foe {
			side = "foe"
		}
		line := fmt.Sprintf("%s[%d] %s  %s", cursor, i+1, t.Fg.Render(label), t.Muted.Render(side))
		body = append(body, line)
	}
	_ = s
	return bv.overlayFrame("Choose a target", width, body)
}

func (bv *battleView) targetLabel(opt battle.TargetOption) string {
	s := bv.state()
	var side *battle.Side
	if opt.Foe {
		side = s.Opponent()
	} else {
		side = s.MySide()
	}
	if side != nil {
		if p := side.ActiveAt(opt.Slot); p != nil {
			return SanitizeLine(p.Name)
		}
	}
	return opt.Spec
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// parseIdentLoose splits a protocol ident such as "p1a: Gengar".
func parseIdentLoose(ident string) (side, slot, name string) {
	i := strings.Index(ident, ": ")
	if i < 0 {
		return "", "", ident
	}
	left := ident[:i]
	name = ident[i+2:]
	if len(left) >= 2 {
		side = left[:2]
		if len(left) > 2 {
			slot = left[2:]
		}
	}
	return side, slot, name
}
