package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/unnipv/pokemon-slowdown/internal/battle"
	"github.com/unnipv/pokemon-slowdown/internal/config"
	"github.com/unnipv/pokemon-slowdown/internal/dex"
	"github.com/unnipv/pokemon-slowdown/internal/showdown"
	"github.com/unnipv/pokemon-slowdown/internal/sprites"
)

// overlayKind selects the battle overlay currently open.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlaySwitch
	overlayInspect
	overlayLog
	overlayChat
	overlayTarget
)

type chatLine struct {
	user string
	text string
	me   bool
	at   int64
}

// spriteRequest is a sprite the battle needs rendered.
type spriteRequest struct {
	Key    string
	Sprite sprites.Ref
}

// battleView owns one battle: its deterministic state plus all UI state.
type battleView struct {
	room    string
	owner   *Model
	theme   Theme
	deps    Deps
	reducer *battle.Reducer

	curLayout LayoutMode

	overlay       overlayKind
	overlayCursor int

	draft           map[int]battle.ChoiceSlot
	slot            int
	pendingMove     int
	pendingMechanic string
	pendingSlot     int
	targets         []battle.TargetOption

	logScroll int
	chat      []chatLine
	chatInput string

	spriteRendered map[string]string
	spriteFrames   map[string]int
	loading        map[string]bool

	previewOrder []int

	animFrame int
	titleText string
	lastError string
}

func newBattleView(room string, owner *Model) *battleView {
	return &battleView{
		room:           room,
		owner:          owner,
		theme:          owner.theme,
		deps:           owner.deps,
		reducer:        battle.NewReducer(room),
		draft:          map[int]battle.ChoiceSlot{},
		spriteRendered: map[string]string{},
		spriteFrames:   map[string]int{},
		loading:        map[string]bool{},
	}
}

// tagline returns a line for a slot using the owner's configuration.
func (bv *battleView) tagline(slot config.TaglineSlot) string {
	if bv.owner == nil {
		return ""
	}
	return bv.owner.cfg.Tagline(slot, bv.owner.cfg.TaglinesRandom)
}

// wonByMe reports whether the battle winner is us.
func (bv *battleView) wonByMe(s *battle.State) bool {
	if s.Winner == "" || bv.owner == nil || bv.owner.username == "" {
		return false
	}
	return showdown.ToID(s.Winner) == showdown.ToID(bv.owner.username)
}

func (bv *battleView) state() *battle.State { return bv.reducer.State }

func (bv *battleView) apply(ev showdown.Event) {
	bv.reducer.Debug = bv.deps.Debug
	if title, ok := ev.(showdown.RoomTitle); ok {
		bv.titleText = SanitizeLine(title.Title)
	}
	bv.reducer.Apply(ev)

	// Reset per-request UI state when a new request arrives.
	if _, ok := ev.(showdown.BattleRequest); ok {
		bv.draft = map[int]battle.ChoiceSlot{}
		bv.slot = 0
		bv.overlay = overlayNone
		bv.overlayCursor = 0
	}
	if e, ok := ev.(showdown.BattleError); ok {
		bv.lastError = SanitizeLine(e.Message)
	}
	if _, ok := ev.(showdown.BattleStart); ok {
		bv.previewOrder = nil
	}
}

func (bv *battleView) title() string {
	s := bv.state()
	if bv.titleText != "" {
		return bv.titleText
	}
	if s.Tier != "" {
		return s.Tier
	}
	return bv.room
}

func (bv *battleView) addChat(user, text string, me bool, at int64) {
	bv.chat = append(bv.chat, chatLine{
		user: SanitizeLine(user),
		text: SanitizeLine(text),
		me:   me,
		at:   at,
	})
	if len(bv.chat) > 500 {
		bv.chat = bv.chat[len(bv.chat)-500:]
	}
}

func (bv *battleView) markLoading(key string) { bv.loading[key] = true }
func (bv *battleView) isLoading(key string) bool {
	return bv.loading[key]
}

func (bv *battleView) receiveSprite(msg spriteReadyMsg) {
	delete(bv.loading, msg.key)
	if msg.err != nil || msg.sprite == nil {
		return
	}
	bv.spriteFrames[msg.key] = 0
	bv.renderSprite(msg.key, msg.sprite)
}

// wantedSprites lists the sprites the current view needs.
func (bv *battleView) wantedSprites(animate bool) []spriteRequest {
	s := bv.state()
	var out []spriteRequest
	add := func(p *battle.Pokemon, back bool) {
		if p == nil || p.Species == "" {
			return
		}
		ref := bv.spriteRef(p, back, animate)
		out = append(out, spriteRequest{Key: ref.Key(), Sprite: ref})
	}
	for _, p := range s.P1.ActiveParty() {
		add(p, s.Me == p.SideID)
	}
	for _, p := range s.P2.ActiveParty() {
		add(p, s.Me == p.SideID)
	}
	if s.TeamPreview {
		for _, p := range s.P1.Party {
			add(p, false)
		}
		for _, p := range s.P2.Party {
			add(p, false)
		}
	}
	return out
}

func (bv *battleView) spriteRef(p *battle.Pokemon, back, animate bool) sprites.Ref {
	id := dex.ToID(p.Species)
	if bv.deps.Dex != nil {
		if sp, ok := bv.deps.Dex.Species(p.Species); ok {
			id = sp.SpriteID()
		}
	}
	return sprites.Ref{ID: id, Shiny: p.Shiny, Back: back, Animated: animate}
}

// renderSprite renders and caches one sprite block.
func (bv *battleView) renderSprite(key string, sprite *sprites.Sprite) {
	if bv.deps.Renderer == nil {
		return
	}
	frame := 0
	if sprite.Animated {
		frame = bv.spriteFrames[key] % sprite.FrameCount()
	}
	cols, rows := bv.layout().SpriteCells()
	if cols == 0 {
		return
	}
	out, err := bv.deps.Renderer.Render(sprite.Frame(frame), cols, rows)
	if err != nil {
		return
	}
	bv.spriteRendered[key] = out
}

// advanceAnimation moves animated sprites to their next frame.
func (bv *battleView) advanceAnimation(global int) {
	if bv.deps.Sprites == nil || bv.deps.Renderer == nil {
		return
	}
	bv.animFrame++
	if bv.animFrame%2 != 0 {
		return
	}
	for key, frame := range bv.spriteFrames {
		ref := bv.refForKey(key)
		sp, ok := bv.deps.Sprites.Cached(ref)
		if !ok || !sp.Animated {
			continue
		}
		bv.spriteFrames[key] = frame + 1
		bv.renderSprite(key, sp)
	}
}

// refForKey reconstructs a sprite ref from its cache key.
func (bv *battleView) refForKey(key string) sprites.Ref {
	parts := strings.Split(key, "|")
	if len(parts) < 2 {
		return sprites.Ref{ID: key}
	}
	kind := parts[1]
	return sprites.Ref{
		ID:       parts[0],
		Back:     strings.Contains(kind, "back"),
		Shiny:    strings.Contains(kind, "shiny"),
		Animated: strings.Contains(kind, "ani"),
	}
}

func (bv *battleView) layout() LayoutMode { return bv.curLayout }

// ---------------------------------------------------------------------------
// Input
// ---------------------------------------------------------------------------

// handleKey processes a key press. It reports whether the key was consumed.
func (bv *battleView) handleKey(msg tea.KeyPressMsg, m *Model) (tea.Cmd, bool) {
	key := msg.String()
	s := bv.state()

	if bv.overlay != overlayNone {
		return bv.handleOverlayKey(key, m), true
	}

	// Team preview takes priority: it is a different interaction entirely.
	if s.TeamPreview {
		return bv.handlePreviewKey(key, m), true
	}

	switch key {
	case "esc":
		m.screen = screenLobby
		return nil, true
	case "?":
		m.helpOpen = true
		return nil, true
	case "l":
		bv.overlay = overlayLog
		bv.logScroll = len(s.Log)
		return nil, true
	case "c":
		bv.overlay = overlayChat
		return nil, true
	case "i":
		bv.overlay = overlayInspect
		return nil, true
	case "s":
		if bv.canSwitch() {
			bv.overlay = overlaySwitch
			bv.overlayCursor = 0
		}
		return nil, true
	case "tab":
		m.nextBattle()
		return nil, true
	}

	// Mechanic toggle applies to the pending move.
	if key == "t" {
		if bv.toggleMechanic() {
			return nil, true
		}
	}

	if n := digit(key); n > 0 {
		return bv.chooseMove(n, m), true
	}
	return nil, false
}

func (bv *battleView) handlePreviewKey(key string, m *Model) tea.Cmd {
	s := bv.state()
	if s.MySide() == nil {
		return nil
	}
	n := len(s.MySide().Party)
	switch key {
	case "enter", " ":
		return bv.submitPreview()
	case "esc":
		// Accept the default order rather than stranding the user.
		bv.previewOrder = nil
		return bv.submitPreview()
	}
	if d := digit(key); d > 0 && d <= n {
		bv.togglePreviewPick(d)
	}
	return nil
}

func (bv *battleView) togglePreviewPick(slot int) {
	found := -1
	for i, v := range bv.previewOrder {
		if v == slot {
			found = i
			break
		}
	}
	if found >= 0 {
		bv.previewOrder = append(bv.previewOrder[:found], bv.previewOrder[found+1:]...)
		return
	}
	bv.previewOrder = append(bv.previewOrder, slot)
}

func (bv *battleView) submitPreview() tea.Cmd {
	s := bv.state()
	if s.MySide() == nil || bv.deps.Client == nil {
		return nil
	}
	n := len(s.MySide().Party)
	order := make([]int, 0, n)
	seen := map[int]bool{}
	for _, v := range bv.previewOrder {
		if !seen[v] {
			order = append(order, v)
			seen[v] = true
		}
	}
	for i := 1; i <= n; i++ {
		if !seen[i] {
			order = append(order, i)
		}
	}
	rqid := 0
	if s.Request != nil {
		rqid = s.Request.RqID
	}
	return func() tea.Msg {
		_ = bv.deps.Client.Choose(bv.room, battle.TeamChoice(order).String(), rqid)
		return nil
	}
}

// canSwitch reports whether a switch is legal right now.
func (bv *battleView) canSwitch() bool {
	s := bv.state()
	req := s.Request
	if req == nil {
		return false
	}
	switch req.Kind() {
	case battle.RequestSwitch, battle.RequestMove:
		return true
	default:
		return false
	}
}

// toggleMechanic enables the primary mechanic for the current slot.
func (bv *battleView) toggleMechanic() bool {
	req := bv.state().Request
	if req == nil || req.Kind() != battle.RequestMove {
		return false
	}
	ar := req.ActiveAt(bv.slot)
	mech, ok := battle.AvailableMechanics(ar).Primary()
	if !ok {
		return false
	}
	if bv.pendingMechanic == mech.Kind {
		bv.pendingMechanic = ""
		return true
	}
	bv.pendingMechanic = mech.Kind
	return true
}

// chooseMove records a move choice for the current slot.
func (bv *battleView) chooseMove(move int, m *Model) tea.Cmd {
	s := bv.state()
	req := s.Request
	if req == nil || req.Kind() != battle.RequestMove {
		return nil
	}
	ar := req.ActiveAt(bv.slot)
	if ar == nil || move < 1 || move > len(ar.Moves) {
		return nil
	}
	mv := ar.Moves[move-1]
	if mv.Disabled.Set {
		return nil
	}

	targets := battle.LegalTargets(mv, bv.slot, bv.activeCount(s.MySide()), bv.activeCount(s.Opponent()))
	if len(targets) > 0 {
		bv.overlay = overlayTarget
		bv.targets = targets
		bv.overlayCursor = 0
		bv.pendingMove = move
		bv.pendingSlot = bv.slot
		return nil
	}
	return bv.commitMove(bv.slot, move, "")
}

func (bv *battleView) activeCount(side *battle.Side) int {
	if side == nil {
		return 0
	}
	return len(side.ActiveParty())
}

// commitMove stores a completed move decision and submits when ready.
func (bv *battleView) commitMove(slot, move int, target string) tea.Cmd {
	bv.draft[slot] = battle.ChoiceSlot{
		Kind:     "move",
		Move:     move,
		Target:   target,
		Mechanic: bv.pendingMechanic,
	}
	bv.pendingMove = 0
	bv.pendingMechanic = ""
	return bv.maybeSubmit()
}

// commitSwitch stores a switch decision and submits when ready.
func (bv *battleView) commitSwitch(partySlot int) tea.Cmd {
	s := bv.state()
	req := s.Request
	if req == nil {
		return nil
	}
	if req.Kind() == battle.RequestSwitch {
		// Forced switch: one decision, submit immediately.
		return bv.sendChoice(battle.Choice{Slots: []battle.ChoiceSlot{{Kind: "switch", Switch: partySlot}}})
	}
	bv.draft[bv.slot] = battle.ChoiceSlot{Kind: "switch", Switch: partySlot}
	return bv.maybeSubmit()
}

// maybeSubmit sends the choice once every required slot has a decision.
func (bv *battleView) maybeSubmit() tea.Cmd {
	s := bv.state()
	req := s.Request
	if req == nil {
		return nil
	}
	need := req.SlotCount()
	slots := make([]battle.ChoiceSlot, 0, need)
	for i := 0; i < need; i++ {
		slot, ok := bv.draft[i]
		if !ok {
			if req.Kind() == battle.RequestMove {
				// A fainted or empty slot may be passed.
				slots = append(slots, battle.ChoiceSlot{Kind: "pass"})
				continue
			}
			return nil
		}
		slots = append(slots, slot)
	}
	return bv.sendChoice(battle.Choice{Slots: slots})
}

func (bv *battleView) sendChoice(choice battle.Choice) tea.Cmd {
	s := bv.state()
	if bv.deps.Client == nil {
		return nil
	}
	rqid := 0
	if s.Request != nil {
		rqid = s.Request.RqID
	}
	text := choice.String()
	bv.draft = map[int]battle.ChoiceSlot{}
	bv.slot = 0
	return func() tea.Msg {
		_ = bv.deps.Client.Choose(bv.room, text, rqid)
		return nil
	}
}

// handleOverlayKey routes keys while an overlay is open.
func (bv *battleView) handleOverlayKey(key string, m *Model) tea.Cmd {
	s := bv.state()
	switch bv.overlay {
	case overlaySwitch:
		slots := bv.switchSlots()
		switch key {
		case "esc", "s":
			bv.overlay = overlayNone
		case "up", "k":
			bv.overlayCursor = clamp(bv.overlayCursor-1, 0, len(slots)-1)
		case "down", "j":
			bv.overlayCursor = clamp(bv.overlayCursor+1, 0, len(slots)-1)
		case "enter":
			if bv.overlayCursor < len(slots) && slots[bv.overlayCursor].Legal {
				cmd := bv.commitSwitch(slots[bv.overlayCursor].Index)
				bv.overlay = overlayNone
				return cmd
			}
		default:
			if d := digit(key); d > 0 {
				for _, sl := range slots {
					if sl.Index == d && sl.Legal {
						cmd := bv.commitSwitch(d)
						bv.overlay = overlayNone
						return cmd
					}
				}
			}
		}
	case overlayTarget:
		switch key {
		case "esc":
			bv.overlay = overlayNone
			bv.pendingMove = 0
		case "up", "k":
			bv.overlayCursor = clamp(bv.overlayCursor-1, 0, len(bv.targets)-1)
		case "down", "j":
			bv.overlayCursor = clamp(bv.overlayCursor+1, 0, len(bv.targets)-1)
		case "enter":
			if bv.overlayCursor < len(bv.targets) {
				t := bv.targets[bv.overlayCursor]
				bv.overlay = overlayNone
				return bv.commitMove(bv.pendingSlot, bv.pendingMove, t.Spec)
			}
		default:
			if d := digit(key); d > 0 && d <= len(bv.targets) {
				t := bv.targets[d-1]
				bv.overlay = overlayNone
				return bv.commitMove(bv.pendingSlot, bv.pendingMove, t.Spec)
			}
		}
	case overlayLog:
		switch key {
		case "esc", "l":
			bv.overlay = overlayNone
		case "up", "k":
			bv.logScroll = clamp(bv.logScroll-1, 0, len(s.Log))
		case "down", "j":
			bv.logScroll = clamp(bv.logScroll+1, 0, len(s.Log))
		case "pgup":
			bv.logScroll = clamp(bv.logScroll-8, 0, len(s.Log))
		case "pgdown":
			bv.logScroll = clamp(bv.logScroll+8, 0, len(s.Log))
		}
	case overlayInspect:
		if key == "esc" || key == "i" {
			bv.overlay = overlayNone
		}
	case overlayChat:
		switch key {
		case "esc":
			bv.overlay = overlayNone
			bv.chatInput = ""
		case "enter":
			text := strings.TrimSpace(bv.chatInput)
			bv.chatInput = ""
			if text != "" && bv.deps.Client != nil {
				room := bv.room
				client := bv.deps.Client
				return func() tea.Msg {
					_ = client.Chat(room, text)
					return nil
				}
			}
		case "backspace":
			if len(bv.chatInput) > 0 {
				bv.chatInput = bv.chatInput[:len(bv.chatInput)-1]
			}
		case "space":
			bv.chatInput += " "
		default:
			if r := msg2rune(key); r != 0 {
				bv.chatInput += string(r)
			}
		}
	}
	return nil
}

// switchSlots returns the party annotated for the switch overlay.
func (bv *battleView) switchSlots() []battle.SwitchSlot {
	s := bv.state()
	side := s.MySide()
	if side == nil {
		return nil
	}
	out := make([]battle.SwitchSlot, 0, len(side.Party))
	for i, p := range side.Party {
		out = append(out, battle.SwitchSlot{
			Index:   i + 1,
			Fainted: p.Fainted,
			Active:  p.Active,
			Legal:   !p.Fainted && !p.Active,
		})
	}
	return out
}

func digit(s string) int {
	if len(s) == 1 && s[0] >= '1' && s[0] <= '9' {
		return int(s[0] - '0')
	}
	return 0
}

// printable returns the rune for a single-character key, or 0.
func printable(s string) rune {
	r := []rune(s)
	if len(r) != 1 {
		return 0
	}
	if r[0] < 0x20 || r[0] == 0x7f {
		return 0
	}
	return r[0]
}

func msg2rune(s string) rune { return printable(s) }

// ---------------------------------------------------------------------------
// Rendering
// ---------------------------------------------------------------------------

func (bv *battleView) render(width, height int, layout LayoutMode) string {
	bv.curLayout = layout
	s := bv.state()
	t := bv.theme

	var lines []string
	lines = append(lines, bv.renderHeader(width, layout))

	switch {
	case s.TeamPreview:
		lines = append(lines, bv.renderPreview(width, layout)...)
	default:
		lines = append(lines, bv.renderField(width, layout)...)
		if s.Ended {
			lines = append(lines, bv.renderResult(width)...)
		}
	}

	if layout.ShowFieldConditions() {
		if cond := bv.renderConditions(); cond != "" {
			lines = append(lines, cond)
		}
	}
	if s.AwaitingChoice && !s.Ended && !s.TeamPreview {
		lines = append(lines, bv.renderMoves(width, layout)...)
	} else if !s.Ended && !s.TeamPreview {
		lines = append(lines, t.Muted.Render(bv.waitingLine()))
	}

	body := strings.Join(lines, "\n")
	if bv.overlay != overlayNone {
		body = bv.renderOverlay(body, width, height, layout)
	}
	return body
}

func (bv *battleView) renderHeader(width int, layout LayoutMode) string {
	s := bv.state()
	t := bv.theme
	left := t.Title.Render(" showdown ")
	mid := t.Muted.Render("· " + SanitizeLine(bv.title()))
	turn := ""
	if s.Turn > 0 {
		turn = t.Fg.Render(fmt.Sprintf("Turn %d", s.Turn))
	}
	timer := ""
	if s.TimerOn {
		timer = t.Warning.Render(" ⏱ timer")
	}
	right := turn + timer
	pad := width - lipgloss.Width(left) - lipgloss.Width(mid) - lipgloss.Width(right) - 2
	if pad < 1 {
		pad = 1
	}
	line := left + mid + strings.Repeat(" ", pad) + right
	return lipgloss.NewStyle().Width(width).MaxWidth(width).Render(line)
}

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

func (bv *battleView) renderResult(width int) []string {
	s := bv.state()
	t := bv.theme
	var out []string
	out = append(out, "")
	if s.Tie {
		out = append(out, t.Warning.Render("  The battle ended in a tie."))
	} else if s.Winner != "" {
		won := bv.wonByMe(s)
		text := "  " + SanitizeLine(s.Winner) + " won the battle!"
		if won {
			out = append(out, t.Success.Render(text))
			out = append(out, t.Muted.Render("  "+bv.tagline(config.SlotVictory)))
		} else {
			out = append(out, t.Danger.Render(text))
			out = append(out, t.Muted.Render("  "+bv.tagline(config.SlotDefeat)))
		}
	}
	out = append(out, "")
	out = append(out, t.Muted.Render("  replay: https://replay.pokemonshowdown.com/"+s.RoomID))
	out = append(out, "")
	out = append(out, t.Dim.Render("  esc back to lobby · tab next battle"))
	return out
}

// renderField draws the battlefield: opponent above, us below.
func (bv *battleView) renderField(width int, layout LayoutMode) []string {
	s := bv.state()
	var out []string

	foe := s.Opponent()
	mine := s.MySide()

	if layout.ShowBattlefield() {
		out = append(out, "")
		out = append(out, bv.renderSideRow(foe, true, width, layout)...)
		out = append(out, "")
		out = append(out, bv.renderSideRow(mine, false, width, layout)...)
		out = append(out, "")
		return out
	}

	// Compact and sidecar: one line per active Pokémon, no big battlefield.
	out = append(out, "")
	if foe != nil {
		out = append(out, bv.renderSideLine(foe, true, width))
	}
	if mine != nil {
		out = append(out, bv.renderSideLine(mine, false, width))
	}
	out = append(out, "")
	return out
}

func (bv *battleView) renderSideRow(side *battle.Side, foe bool, width int, layout LayoutMode) []string {
	if side == nil {
		return nil
	}
	t := bv.theme
	active := side.ActiveParty()
	var out []string

	label := side.Name
	if foe {
		label = "opponent · " + side.Name
	}
	out = append(out, t.Muted.Render("  "+SanitizeLine(label)))

	for _, p := range active {
		sprite := bv.spriteBlock(p, layout)
		info := bv.renderPokemonInfo(p, width-24, foe)
		row := lipgloss.JoinHorizontal(lipgloss.Top, "  ", sprite, "  ", info)
		out = append(out, row)
	}
	return out
}

func (bv *battleView) renderSideLine(side *battle.Side, foe bool, width int) string {
	t := bv.theme
	active := side.ActiveParty()
	if len(active) == 0 {
		return t.Dim.Render("  (no active Pokémon)")
	}
	var parts []string
	for _, p := range active {
		parts = append(parts, bv.renderCompactPokemon(p, width/len(active), foe))
	}
	return strings.Join(parts, "  ")
}

// spriteBlock returns the rendered sprite for a Pokémon, or a dim placeholder
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

// placeholderBlock is a quiet stand-in that keeps the layout stable while a
// sprite downloads.
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

func (bv *battleView) renderCompactPokemon(p *battle.Pokemon, width int, foe bool) string {
	t := bv.theme
	name := SanitizeLine(p.Name)
	if p.Fainted {
		name = t.Dim.Render(name + " (fainted)")
	} else {
		name = t.Fg.Bold(true).Render(name)
	}
	bar := bv.hpBar(p.HPPercent, 10)
	status := bv.statusBadge(p.Status)
	line := name + " " + bar
	if status != "" {
		line += " " + status
	}
	if boosts := bv.boostText(p); boosts != "" {
		line += " " + t.Accent.Render(boosts)
	}
	return line
}

func (bv *battleView) renderPokemonInfo(p *battle.Pokemon, width int, foe bool) string {
	t := bv.theme
	if width < 12 {
		width = 12
	}
	name := SanitizeLine(p.Name)
	if p.Gender != "" {
		name += " " + genderGlyph(p.Gender)
	}
	title := t.Fg.Bold(true).Render(name)
	if p.Fainted {
		title = t.Dim.Render(name + "  fainted")
	}

	bar := bv.hpBar(p.HPPercent, 16)
	hp := t.Muted.Render(fmt.Sprintf("%d%%", p.HPPercent))
	if p.MaxHP > 0 && !foe {
		hp = t.Muted.Render(fmt.Sprintf("%d/%d", p.HP, p.MaxHP))
	}

	var extra []string
	if s := bv.statusBadge(p.Status); s != "" {
		extra = append(extra, s)
	}
	if p.Terastallized {
		extra = append(extra, t.Primary.Render("tera:"+p.TeraType))
	}
	if b := bv.boostText(p); b != "" {
		extra = append(extra, t.Accent.Render(b))
	}
	if p.Item != "" && !foe {
		extra = append(extra, t.Muted.Render(p.Item))
	}

	lines := []string{title, bar + " " + hp}
	if len(extra) > 0 {
		lines = append(lines, strings.Join(extra, "  "))
	}
	return strings.Join(lines, "\n")
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

func (bv *battleView) boostText(p *battle.Pokemon) string {
	var parts []string
	for _, stat := range []string{"atk", "def", "spa", "spd", "spe"} {
		if n := p.Boosts[stat]; n != 0 {
			sign := "+"
			if n < 0 {
				sign = ""
			}
			parts = append(parts, fmt.Sprintf("%s%s%d", strings.ToUpper(stat), sign, n))
		}
	}
	return strings.Join(parts, " ")
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

// renderMoves draws the move panel. This is where most turns are decided, so it
// stays readable at every width.
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

	var out []string
	out = append(out, "")
	if len(req.Active) > 1 {
		out = append(out, t.Muted.Render(fmt.Sprintf("  choosing for slot %d of %d", bv.slot+1, len(req.Active))))
	}

	mech := battle.AvailableMechanics(ar)
	for i, mv := range ar.Moves {
		out = append(out, bv.renderMoveLine(i+1, mv, mech, width, layout))
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

func (bv *battleView) renderMoveLine(n int, mv battle.MoveRequest, mech battle.Mechanics, width int, layout LayoutMode) string {
	t := bv.theme

	typeBadge := ""
	if bv.deps.Dex != nil {
		if m, ok := bv.deps.Dex.Move(mv.ID); ok {
			if st, ok := t.Types[strings.ToLower(m.Type)]; ok {
				typeBadge = st.Render(strings.ToUpper(m.Type))
			} else {
				typeBadge = t.Muted.Render(strings.ToUpper(m.Type))
			}
		}
	}

	pp := t.Muted.Render(fmt.Sprintf("%d/%d", mv.PP, mv.MaxPP))
	var name string
	if mv.Disabled.Set {
		name = t.Disabled.Render(mv.Move + " (disabled)")
	} else {
		name = t.Fg.Render(mv.Move)
	}

	key := t.Accent.Bold(true).Render(fmt.Sprintf("%d", n))
	left := fmt.Sprintf("  %s %s", key, name)
	if typeBadge != "" && layout != LayoutCompact {
		left += "  " + typeBadge
	}

	if layout == LayoutCompact {
		return left + "  " + pp
	}
	pad := width - lipgloss.Width(left) - lipgloss.Width(pp) - 2
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + pp
}

func (bv *battleView) renderPreview(width int, layout LayoutMode) []string {
	s := bv.state()
	t := bv.theme
	var out []string
	out = append(out, "")
	out = append(out, t.Title.Render("  Choose your lead order"))
	out = append(out, t.Muted.Render("  press a number to bring a Pokémon forward, enter to confirm"))
	out = append(out, "")

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
		marker := " "
		if n, ok := picks[slot]; ok {
			marker = t.Accent.Render(fmt.Sprintf("%d", n))
		}
		name := t.Fg.Render(SanitizeLine(p.Name))
		types := bv.typeList(p.Species)
		out = append(out, fmt.Sprintf("  [%s] %s %s", marker, name, types))
	}
	out = append(out, "")
	return out
}

func (bv *battleView) typeList(species string) string {
	if bv.deps.Dex == nil {
		return ""
	}
	sp, ok := bv.deps.Dex.Species(species)
	if !ok {
		return ""
	}
	var parts []string
	for _, typ := range sp.Types {
		if st, ok := bv.theme.Types[strings.ToLower(typ)]; ok {
			parts = append(parts, st.Render(strings.ToUpper(typ)))
		}
	}
	return strings.Join(parts, " ")
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
