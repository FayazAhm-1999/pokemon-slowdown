// Package tui renders the pokemon slowdown interface. It consumes typed
// showdown events and battle state and never parses protocol strings itself.
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/unnipv/pokemon-slowdown/internal/config"
	"github.com/unnipv/pokemon-slowdown/internal/dex"
	"github.com/unnipv/pokemon-slowdown/internal/notify"
	"github.com/unnipv/pokemon-slowdown/internal/showdown"
	"github.com/unnipv/pokemon-slowdown/internal/sprites"
	"github.com/unnipv/pokemon-slowdown/internal/teams"
)

// Deps are the collaborators the UI needs. All of them may be nil in tests.
type Deps struct {
	Client   *showdown.Client
	Dex      *dex.Dex
	Sprites  *sprites.Manager
	Renderer sprites.Renderer
	Notifier *notify.Notifier
	Teams    *teams.Store
	Debug    bool
	// Logf writes a debug line to the log file, if debug logging is on.
	Logf func(string, ...any)
	// Now allows tests to control time.
	Now func() time.Time
}

// screen identifies the active top-level screen.
type screen int

const (
	screenLobby screen = iota
	screenBattle
	screenTeams
)

// connState is the websocket state shown in the status line.
type connState int

const (
	connConnecting connState = iota
	connOnline
	connReconnecting
	connOffline
)

func (c connState) label() string {
	switch c {
	case connOnline:
		return "online"
	case connReconnecting:
		return "reconnecting"
	case connOffline:
		return "offline"
	default:
		return "connecting"
	}
}

// Model is the root Bubble Tea model.
type Model struct {
	cfg   config.Config
	deps  Deps
	theme Theme

	width, height int
	layout        LayoutMode

	screen   screen
	conn     connState
	username string
	loggedIn bool
	authErr  string
	challstr bool

	formats      []showdown.Format
	formatFilter string
	formatCursor int
	formatScroll int

	searching      []string
	games          map[string]string
	challengesFrom map[string]string
	challengeTo    *showdown.Challenge

	battles map[string]*battleView
	order   []string
	active  string

	palette       paletteState
	prompt        promptState
	helpOpen      bool
	confirm       string
	confirmAction string
	teamCursor    int
	toast         string
	toastAt       time.Time
	now           time.Time

	pendingBattle string
	animFrame     int

	// one-shot startup actions
	autoQueue        string
	autoDone         bool
	pendingSpectate  string
	pendingChallenge string
}

// SetAutoQueue queues a ladder search for a format as soon as the client is
// ready. This backs the "slowdown rand" shortcut.
func (m *Model) SetAutoQueue(format string) { m.autoQueue = format }

// SetPendingSpectate joins a battle room as a spectator once connected.
func (m *Model) SetPendingSpectate(id string) { m.pendingSpectate = id }

// SetPendingChallenge challenges a user once connected.
func (m *Model) SetPendingChallenge(user string) { m.pendingChallenge = user }

// SetStartScreenTeams opens the team manager on launch.
func (m *Model) SetStartScreenTeams() { m.screen = screenTeams }

// New builds the root model.
func New(cfg config.Config, deps Deps) *Model {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	m := &Model{
		cfg:            cfg,
		deps:           deps,
		theme:          LoadTheme(cfg.Theme),
		conn:           connConnecting,
		games:          map[string]string{},
		challengesFrom: map[string]string{},
		battles:        map[string]*battleView{},
		now:            deps.Now(),
		layout:         LayoutFor(80),
	}
	return m
}

// ---------------------------------------------------------------------------
// Messages
// ---------------------------------------------------------------------------

type tickMsg time.Time

type dexReadyMsg struct{ err error }

type spriteReadyMsg struct {
	key    string
	sprite *sprites.Sprite
	err    error
}

// Init starts the event pump, the dex load and the animation ticker.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tick()}
	if m.deps.Client != nil {
		cmds = append(cmds, waitForEvent(m.deps.Client.Events()))
	}
	if m.deps.Dex != nil && !m.deps.Dex.Loaded() {
		d := m.deps.Dex
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			return dexReadyMsg{err: d.Ensure(ctx)}
		})
	}
	if m.cfg.DefaultFormat != "" && len(m.cfg.Keybindings) == 0 {
		// nothing: default format is used by the CLI shortcut, not forced here
	}
	return tea.Batch(cmds...)
}

func tick() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func waitForEvent(ch <-chan showdown.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return nil
		}
		return ev
	}
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout = LayoutFor(msg.Width)
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.PasteMsg:
		// Pasted content can be multi-line, which is how teams are imported.
		if m.prompt.open {
			m.prompt.input += Sanitize(msg.Content)
		}
		return m, nil

	case tickMsg:
		m.now = m.deps.Now()
		if m.toast != "" && m.now.Sub(m.toastAt) > 6*time.Second {
			m.toast = ""
		}
		m.animFrame++
		cmds := []tea.Cmd{tick()}
		if cmd := m.animateSprites(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case dexReadyMsg:
		if msg.err != nil {
			m.setToast("Could not load dex data: " + msg.err.Error())
		}
		return m, nil

	case spriteReadyMsg:
		if bv := m.activeBattle(); bv != nil {
			bv.receiveSprite(msg)
		}
		return m, nil
	}

	if ev, ok := msg.(showdown.Event); ok {
		cmds := m.handleShowdownEvent(ev)
		// Keep listening.
		if m.deps.Client != nil {
			cmds = append(cmds, waitForEvent(m.deps.Client.Events()))
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

// handleShowdownEvent routes a typed server event.
func (m *Model) handleShowdownEvent(ev showdown.Event) []tea.Cmd {
	var cmds []tea.Cmd
	if m.deps.Logf != nil && m.deps.Debug {
		m.deps.Logf("event %T room=%q", ev, ev.Room())
	}

	switch e := ev.(type) {
	case showdown.Connected:
		m.conn = connOnline
		if e.Reconnected {
			m.setToast("Reconnected.")
		}
		if m.pendingSpectate != "" && m.deps.Client != nil {
			id := m.pendingSpectate
			m.pendingSpectate = ""
			client := m.deps.Client
			m.setToast("Joining " + id + "…")
			cmds = append(cmds, func() tea.Msg { _ = client.JoinRoom(id); return nil })
		}
	case showdown.Disconnected:
		m.conn = connReconnecting
	case showdown.Reconnecting:
		m.conn = connReconnecting
	case showdown.ChallStr:
		m.challstr = true
	case showdown.AuthFailed:
		m.authErr = e.Err.Error()
		m.setToast("Login failed: " + e.Err.Error())
	case showdown.UpdateUser:
		m.username = e.Name
		m.loggedIn = e.Named
		if e.Named {
			m.setToast("Logged in as " + e.Name)
		}
	case showdown.Popup:
		m.setToast(SanitizeLine(e.Message))
	case showdown.Notify:
		title := SanitizeLine(e.Title)
		body := SanitizeLine(e.Message)
		m.setToast(strings.TrimSpace(title + " " + body))
		if m.deps.Notifier != nil {
			m.deps.Notifier.Notify(title, body)
		}
	case showdown.PM:
		// Replies to roomless commands — including every command error — come
		// back as a PM from the server rather than as a room message. Dropping
		// these makes failures look like nothing happened.
		sender := SanitizeLine(e.Sender)
		body := SanitizeLine(e.Message)
		if sender == "" || sender == "~" {
			m.setToast(body)
			break
		}
		m.setToast("PM from " + sender + ": " + body)
		if m.deps.Notifier != nil {
			m.deps.Notifier.Notify("PM from "+sender, body)
		}
	case showdown.NameTaken:
		m.setToast("Name taken: " + SanitizeLine(e.Message))
	case showdown.FormatsUpdated:
		m.formats = e.Formats
		if m.autoQueue != "" && !m.autoDone {
			m.autoDone = true
			format := m.autoQueue
			random := strings.Contains(format, "random")
			cmds = append(cmds, m.queueFormat(showdown.Format{
				ID: format, Random: random, Searchable: true, Challengeable: true,
			}))
		}
		if m.pendingChallenge != "" && m.deps.Client != nil {
			user := m.pendingChallenge
			m.pendingChallenge = ""
			client := m.deps.Client
			format := m.cfg.DefaultFormat
			m.setToast("Challenging " + user + "…")
			cmds = append(cmds, func() tea.Msg { _ = client.Challenge(user, format); return nil })
		}
	case showdown.SearchUpdated:
		m.searching = e.Searching
		if e.Games != nil {
			m.games = e.Games
		}
		if m.pendingBattle != "" {
			if _, ok := m.games[m.pendingBattle]; ok {
				m.pendingBattle = ""
			}
		}
	case showdown.ChallengesUpdated:
		m.challengesFrom = e.ChallengesFrom
		m.challengeTo = e.ChallengeTo
	case showdown.BattleStarted:
		if e.Room() != "" {
			m.deps.Client.RememberRoom(e.Room())
		}
	case showdown.RoomInit:
		if e.Type == "battle" {
			m.deps.Client.RememberRoom(e.RoomID)
			bv := m.battleFor(e.RoomID)
			bv.apply(ev)
			m.openBattle(e.RoomID)
		}
	case showdown.RoomTitle:
		if bv := m.battles[e.Room()]; bv != nil {
			bv.apply(ev)
		}
	case showdown.BattlePlayer, showdown.BattleTeamSize, showdown.BattleGameType,
		showdown.BattleGen, showdown.BattleTier, showdown.BattleRated, showdown.BattleRule,
		showdown.BattleClearpoke, showdown.BattlePoke, showdown.BattleTeamPreview,
		showdown.BattleStart, showdown.BattleTurn, showdown.BattleRequest, showdown.BattleWin,
		showdown.BattleTie, showdown.BattleInactive, showdown.BattleUpkeep, showdown.BattleTimestamp,
		showdown.BattleError, showdown.BattleClear, showdown.BattleMessage, showdown.BattleMove,
		showdown.BattleSwitch, showdown.BattleDetailsChange, showdown.BattleFormeChange,
		showdown.BattleReplace, showdown.BattleSwap, showdown.BattleCant, showdown.BattleFaint,
		showdown.BattleDamage, showdown.BattleHeal, showdown.BattleSetHP, showdown.BattleStatus,
		showdown.BattleCureStatus, showdown.BattleCureTeam, showdown.BattleBoost,
		showdown.BattleSwapBoost, showdown.BattleInvertBoost, showdown.BattleClearBoost,
		showdown.BattleClearAllBoost, showdown.BattleCopyBoost, showdown.BattleWeather,
		showdown.BattleFieldStart, showdown.BattleFieldEnd, showdown.BattleFieldActivate,
		showdown.BattleSideStart, showdown.BattleSideEnd, showdown.BattleSwapSideConditions,
		showdown.BattleVolatileStart, showdown.BattleVolatileEnd, showdown.BattleItem,
		showdown.BattleEndItem, showdown.BattleAbility, showdown.BattleEndAbility,
		showdown.BattleTransform, showdown.BattleMega, showdown.BattlePrimal, showdown.BattleBurst,
		showdown.BattleZPower, showdown.BattleZBroken, showdown.BattleHint, showdown.BattleEffect,
		showdown.BattleUnknown:
		room := ev.Room()
		if room == "" {
			break
		}
		bv := m.battleFor(room)
		if m.active == "" {
			// First battle we see becomes the focused one.
			m.openBattle(room)
		}
		wasAwaiting := bv.state().AwaitingChoice
		bv.apply(ev)
		cmds = append(cmds, m.afterBattleEvent(bv, wasAwaiting)...)
	case showdown.ChatMessage:
		room := ev.Room()
		if bv := m.battles[room]; bv != nil {
			bv.addChat(e.User, e.Message, m.isMe(e.User), e.Time)
		}
	}
	return cmds
}

// afterBattleEvent reacts to a battle state change: notifications when it
// becomes our turn, and sprite loads for anything newly visible.
func (m *Model) afterBattleEvent(bv *battleView, wasAwaiting bool) []tea.Cmd {
	var cmds []tea.Cmd
	nowAwaiting := bv.state().AwaitingChoice

	if nowAwaiting && !wasAwaiting {
		// Only notify when this battle is not the focused one, so we never
		// spam while the user is already looking at it.
		if m.active != bv.room || m.screen != screenBattle {
			if m.deps.Notifier != nil {
				m.deps.Notifier.Notify("Your turn!", "Turn "+fmt.Sprint(bv.state().Turn))
			}
			m.setToast("Your turn in " + bv.title())
		}
	}
	cmds = append(cmds, m.loadSprites(bv)...)
	return cmds
}

// isMe reports whether a username is ours.
func (m *Model) isMe(user string) bool {
	if m.username == "" {
		return false
	}
	return showdown.ToID(user) == showdown.ToID(m.username)
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

// View implements tea.Model.
func (m *Model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.WindowTitle = "pokemon slowdown"
	return v
}

func (m *Model) render() string {
	if TooSmall(m.width, m.height) {
		return m.renderTooSmall()
	}

	var body string
	switch m.screen {
	case screenBattle:
		if bv := m.activeBattle(); bv != nil {
			body = bv.render(m.width, m.height-2, m.layout)
		} else {
			body = m.theme.Muted.Render("No battle selected. Press esc to return to the lobby.")
		}
	case screenTeams:
		body = m.renderTeams(m.width, m.height-2)
	default:
		body = m.renderLobby(m.width, m.height-2)
	}

	status := m.renderStatus()
	out := body
	// The toast gets its own line so it cannot be clipped off the end of an
	// already-full status line.
	if toast := m.renderToast(); toast != "" {
		out += "\n" + toast
	}
	out += "\n" + status
	if m.helpOpen {
		out = m.overlayBox(out, m.renderHelp())
	}
	if m.palette.open {
		out = m.overlayBox(out, m.renderPalette())
	}
	if m.prompt.open {
		out = m.overlayBox(out, m.renderPrompt())
	}
	if m.confirm != "" {
		out = m.overlayBox(out, m.renderConfirm())
	}
	return out
}

func (m *Model) renderTooSmall() string {
	msg := fmt.Sprintf("Terminal too small\n%d×%d (need %d×%d)\nResize to continue.",
		m.width, m.height, MinWidth, MinHeight)
	return m.theme.Muted.Render(msg)
}

// renderStatus is the contextual status line.
func (m *Model) renderStatus() string {
	left := m.theme.Dim.Render("esc lobby")
	if m.screen == screenBattle {
		left = m.theme.Dim.Render("1-4 move  s switch  i inspect  l log  c chat  : palette")
		if m.layout == LayoutCompact {
			left = m.theme.Dim.Render("1-4 move  s switch  : palette")
		}
	}
	if m.palette.open || m.helpOpen {
		left = m.theme.Dim.Render("esc close")
	}

	conn := m.theme.Muted.Render(m.conn.label())
	if m.conn == connOnline {
		conn = m.theme.Success.Render(m.conn.label())
	} else if m.conn == connOffline {
		conn = m.theme.Danger.Render(m.conn.label())
	} else if m.conn == connReconnecting {
		conn = m.theme.Warning.Render(m.conn.label())
	}

	name := m.username
	if name == "" {
		name = "guest"
	}
	right := m.theme.Muted.Render(SanitizeLine(name)) + "  " + conn
	if len(m.searching) > 0 {
		right = m.theme.Accent.Render("searching "+strings.Join(m.searching, ",")) + "  " + right
	}

	pad := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	return left + strings.Repeat(" ", pad) + right
}

func (m *Model) renderToast() string {
	if m.toast == "" {
		return ""
	}
	return m.theme.Accent.Render(" " + m.toast + " ")
}

func (m *Model) renderHelp() string {
	lines := []string{
		m.theme.Title.Render("keyboard"),
		"",
		"  1-4        choose a move",
		"  s          switch (then 1-6, or a name)",
		"  t          use the current mechanic (Tera/Mega/Z/Dynamax)",
		"  i          inspect the selected Pokémon",
		"  l          battle log",
		"  c          battle chat",
		"  tab        next battle",
		"  :          command palette",
		"  ?          this help",
		"  esc        close / back to lobby",
		"  q          quit (asks first during a battle)",
		"  ctrl+c     quit immediately",
		"",
		m.theme.Muted.Render("Taglines: " + m.cfg.Tagline(config.SlotIdle, false)),
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderConfirm() string {
	return m.theme.Danger.Render(m.confirm) + "\n\n" +
		m.theme.Muted.Render("enter / y to confirm   esc / n to cancel")
}

// overlayBox centres a box over the current content.
func (m *Model) overlayBox(base, box string) string {
	boxW := lipgloss.Width(box)
	boxH := lipgloss.Height(box)
	_ = boxW
	_ = boxH
	// Keep it simple and predictable: place the overlay near the top, centred.
	pad := (m.width - boxW) / 2
	if pad < 0 {
		pad = 0
	}
	indented := indentBlock(box, pad)
	lines := strings.Split(base, "\n")
	overlay := strings.Split(indented, "\n")
	out := make([]string, 0, len(lines))
	for i, ln := range lines {
		if i < len(overlay) {
			out = append(out, overlay[i])
		} else {
			out = append(out, ln)
		}
	}
	return strings.Join(out, "\n")
}

func (m *Model) setToast(s string) {
	m.toast = SanitizeLine(s)
	m.toastAt = m.deps.Now()
}

// ---------------------------------------------------------------------------
// Battle plumbing
// ---------------------------------------------------------------------------

func (m *Model) battleFor(room string) *battleView {
	if bv, ok := m.battles[room]; ok {
		return bv
	}
	bv := newBattleView(room, m)
	m.battles[room] = bv
	m.order = append(m.order, room)
	return bv
}

func (m *Model) activeBattle() *battleView {
	if m.active == "" {
		return nil
	}
	return m.battles[m.active]
}

func (m *Model) openBattle(room string) {
	if _, ok := m.battles[room]; !ok {
		m.battleFor(room)
	}
	m.active = room
	m.screen = screenBattle
}

// nextBattle cycles to the next battle that needs input, then to the next one.
func (m *Model) nextBattle() {
	if len(m.order) == 0 {
		return
	}
	// Prefer a battle awaiting a choice.
	for _, room := range m.order {
		bv := m.battles[room]
		if bv != nil && bv.state().AwaitingChoice && !bv.state().Ended && room != m.active {
			m.openBattle(room)
			return
		}
	}
	idx := 0
	for i, room := range m.order {
		if room == m.active {
			idx = i
			break
		}
	}
	next := m.order[(idx+1)%len(m.order)]
	m.openBattle(next)
}

// removeBattle drops a finished battle.
func (m *Model) removeBattle(room string) {
	delete(m.battles, room)
	out := m.order[:0]
	for _, r := range m.order {
		if r != room {
			out = append(out, r)
		}
	}
	m.order = out
	if m.active == room {
		m.active = ""
		if len(m.order) > 0 {
			m.active = m.order[0]
		} else {
			m.screen = screenLobby
		}
	}
}

// ---------------------------------------------------------------------------
// Sprites
// ---------------------------------------------------------------------------

// loadSprites queues loads for any sprite the active battle needs but has not
// got yet. Loading never blocks input: the UI shows a placeholder until the
// spriteReadyMsg arrives.
func (m *Model) loadSprites(bv *battleView) []tea.Cmd {
	if m.deps.Sprites == nil || m.deps.Renderer == nil {
		return nil
	}
	var cmds []tea.Cmd
	for _, ref := range bv.wantedSprites(m.cfg.Sprites.Animate) {
		if _, ok := m.deps.Sprites.Cached(ref.Sprite); ok {
			continue
		}
		if m.deps.Sprites.Failed(ref.Sprite) {
			continue
		}
		if bv.isLoading(ref.Key) {
			continue
		}
		bv.markLoading(ref.Key)
		ref := ref
		cmds = append(cmds, func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			sp, err := m.deps.Sprites.Load(ctx, ref.Sprite)
			return spriteReadyMsg{key: ref.Key, sprite: sp, err: err}
		})
	}
	return cmds
}

// animateSprites re-renders animated sprites on the tick. It is a no-op when
// animation is off or the active screen shows no sprites.
func (m *Model) animateSprites() tea.Cmd {
	if !m.cfg.Sprites.Animate || m.screen != screenBattle {
		return nil
	}
	if !m.layout.Sprites() {
		return nil
	}
	if bv := m.activeBattle(); bv != nil {
		bv.advanceAnimation(m.animFrame)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func indentBlock(block string, pad int) string {
	if pad <= 0 {
		return block
	}
	prefix := strings.Repeat(" ", pad)
	lines := strings.Split(block, "\n")
	for i, ln := range lines {
		if ln == "" {
			continue
		}
		lines[i] = prefix + ln
	}
	return strings.Join(lines, "\n")
}
