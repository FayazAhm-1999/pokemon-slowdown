package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/unnipv/pokemon-slowdown/internal/showdown"
	"github.com/unnipv/pokemon-slowdown/internal/teams"
)

// handleKey routes a key press for the whole application.
func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.palette.open {
		return m, m.handlePaletteKey(key)
	}
	if m.prompt.open {
		return m, m.handlePromptKey(key)
	}
	if m.confirm != "" {
		return m, m.handleConfirmKey(key)
	}
	if m.helpOpen {
		if key == "esc" || key == "?" || key == "q" {
			m.helpOpen = false
		}
		return m, nil
	}

	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case ":":
		m.openPalette()
		return m, nil
	case "?":
		m.helpOpen = true
		return m, nil
	case "tab":
		if m.screen == screenBattle {
			m.nextBattle()
		}
		return m, nil
	}

	switch m.screen {
	case screenBattle:
		if bv := m.activeBattle(); bv != nil {
			cmd, handled := bv.handleKey(msg, m)
			if handled {
				return m, cmd
			}
		}
		if key == "q" {
			m.confirm = "Quit pokemon slowdown?"
			m.confirmAction = "quit"
			return m, nil
		}
	case screenTeams:
		return m, m.handleTeamsKey(key)
	default:
		return m, m.handleLobbyKey(key)
	}
	return m, nil
}

func (m *Model) handleConfirmKey(key string) tea.Cmd {
	switch key {
	case "enter", "y":
		action := m.confirmAction
		m.confirm = ""
		m.confirmAction = ""
		switch action {
		case "quit":
			return tea.Quit
		case "forfeit":
			if bv := m.activeBattle(); bv != nil && m.deps.Client != nil {
				room := bv.room
				client := m.deps.Client
				m.removeBattle(room)
				return func() tea.Msg {
					_ = client.SendTo(room, "/forfeit")
					return nil
				}
			}
		}
	case "esc", "n", "q":
		m.confirm = ""
		m.confirmAction = ""
	}
	return nil
}

// ---------------------------------------------------------------------------
// Lobby
// ---------------------------------------------------------------------------

func (m *Model) handleLobbyKey(key string) tea.Cmd {
	items := m.filteredFormats()
	switch key {
	case "up", "k":
		m.formatCursor = clamp(m.formatCursor-1, 0, max(0, len(items)-1))
	case "down", "j":
		m.formatCursor = clamp(m.formatCursor+1, 0, max(0, len(items)-1))
	case "enter":
		if m.formatCursor < len(items) {
			return m.queueFormat(items[m.formatCursor])
		}
	case "backspace":
		if m.formatFilter != "" {
			m.formatFilter = m.formatFilter[:len(m.formatFilter)-1]
			m.formatCursor = 0
		}
	case "esc":
		m.formatFilter = ""
		m.formatCursor = 0
	case "t":
		m.screen = screenTeams
	default:
		if r := printable(key); r != 0 && r != ' ' {
			m.formatFilter += string(r)
			m.formatCursor = 0
		} else if key == "space" {
			m.formatFilter += " "
			m.formatCursor = 0
		}
	}
	return nil
}

// formatLabel returns a format's display name, preferring the name the server
// sent over the curated fallback table.
func formatLabel(f showdown.Format) string {
	if f.Name != "" {
		return f.Name
	}
	return FormatName(f.ID)
}

// filteredFormats applies the fuzzy filter to the server's format list.
func (m *Model) filteredFormats() []showdown.Format {
	if m.formatFilter == "" {
		return m.formats
	}
	names := make([]string, len(m.formats))
	for i, f := range m.formats {
		names[i] = formatLabel(f)
	}
	matches := fuzzy.Find(m.formatFilter, names)
	out := make([]showdown.Format, 0, len(matches))
	for _, mt := range matches {
		out = append(out, m.formats[mt.Index])
	}
	return out
}

// queueFormat uploads the right team and starts a ladder search.
func (m *Model) queueFormat(f showdown.Format) tea.Cmd {
	if m.deps.Client == nil {
		return nil
	}
	client := m.deps.Client
	format := f.ID
	team := m.teamForFormat(f)
	m.pendingBattle = ""
	m.setToast("Searching " + formatLabel(f) + "…")
	return func() tea.Msg {
		if team != "" {
			_ = client.SendTeam(team)
		} else {
			_ = client.SendTeam("null")
		}
		_ = client.Search(format)
		return nil
	}
}

// teamForFormat picks a stored team to submit. Random formats need none.
func (m *Model) teamForFormat(f showdown.Format) string {
	if f.Random || m.deps.Teams == nil {
		return ""
	}
	all := m.deps.Teams.All()
	if len(all) == 0 {
		return ""
	}
	// Prefer an exact format match, else the first team.
	for _, t := range all {
		if t.Format == f.ID {
			return t.Pack()
		}
	}
	return all[0].Pack()
}

func (m *Model) renderLobby(width, height int) string {
	if m.layout == LayoutCompact {
		return m.renderLobbyList(width, height)
	}

	listWidth := width * 3 / 5
	if listWidth < 30 {
		listWidth = 30
	}
	sideWidth := width - listWidth - 2

	left := m.renderLobbyList(listWidth, height)
	right := m.renderLobbySide(sideWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m *Model) renderLobbyList(width, height int) string {
	t := m.theme
	items := m.filteredFormats()

	var b strings.Builder
	b.WriteString(t.Title.Render("Find a battle") + "\n")
	filter := m.formatFilter
	if filter == "" {
		filter = t.Dim.Render("search formats…")
	} else {
		filter = t.Fg.Render(filter) + t.Primary.Render("▌")
	}
	b.WriteString(t.Muted.Render("  / ") + filter + "\n\n")

	rows := height - 5
	if rows < 3 {
		rows = 3
	}
	if m.formatCursor < m.formatScroll {
		m.formatScroll = m.formatCursor
	}
	if m.formatCursor >= m.formatScroll+rows {
		m.formatScroll = m.formatCursor - rows + 1
	}
	end := m.formatScroll + rows
	if end > len(items) {
		end = len(items)
	}
	if len(items) == 0 {
		b.WriteString(t.Muted.Render("  no formats matched\n"))
	}
	for i := m.formatScroll; i < end; i++ {
		f := items[i]
		name := formatLabel(f)
		line := fmt.Sprintf("  %-34s", truncate(name, 34))
		tags := ""
		if f.Random {
			tags += t.Accent.Render("random ")
		}
		if !f.Searchable {
			tags += t.Dim.Render("challenge ")
		}
		line += tags
		if i == m.formatCursor {
			b.WriteString(t.Selected.Width(width-2).Render("▸ "+truncate(name, width-6)+" ") + "\n")
			continue
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m *Model) renderLobbySide(width, height int) string {
	t := m.theme
	if width < 10 {
		return ""
	}
	var b strings.Builder
	b.WriteString(t.Title.Render("Status") + "\n\n")

	if len(m.searching) > 0 {
		b.WriteString(t.Accent.Render("● searching "+strings.Join(m.searching, ", ")) + "\n")
		b.WriteString(t.Dim.Render("  : cancel search") + "\n\n")
	}

	if len(m.games) > 0 {
		b.WriteString(t.Muted.Render("Battles") + "\n")
		rooms := make([]string, 0, len(m.games))
		for r := range m.games {
			rooms = append(rooms, r)
		}
		sort.Strings(rooms)
		for _, r := range rooms {
			title := SanitizeLine(m.games[r])
			b.WriteString("  " + t.Fg.Render(truncate(title, width-4)) + "\n")
		}
		b.WriteString("\n")
	}

	if len(m.challengesFrom) > 0 {
		b.WriteString(t.Warning.Render("Challenges") + "\n")
		for user, format := range m.challengesFrom {
			b.WriteString("  " + t.Fg.Render(SanitizeLine(user)) + " " +
				t.Muted.Render(FormatName(format)) + "\n")
		}
		b.WriteString(t.Dim.Render("  : accept / reject") + "\n\n")
	}
	if m.challengeTo != nil {
		b.WriteString(t.Muted.Render("Challenging "+SanitizeLine(m.challengeTo.To)+"…") + "\n\n")
	}

	if m.conn != connOnline {
		b.WriteString(t.Warning.Render("connection: "+m.conn.label()) + "\n")
	}
	if m.authErr != "" {
		b.WriteString(t.Danger.Render(truncate(m.authErr, width-2)) + "\n")
	}

	b.WriteString("\n" + t.Muted.Render("enter queue · t teams · : palette · ? help"))
	return b.String()
}

// ---------------------------------------------------------------------------
// Teams
// ---------------------------------------------------------------------------

func (m *Model) handleTeamsKey(key string) tea.Cmd {
	switch key {
	case "esc":
		m.screen = screenLobby
	case "up", "k":
		m.teamCursor = clamp(m.teamCursor-1, 0, max(0, len(m.teamsList())-1))
	case "down", "j":
		m.teamCursor = clamp(m.teamCursor+1, 0, max(0, len(m.teamsList())-1))
	}
	return nil
}

func (m *Model) teamsList() []teams.Team {
	if m.deps.Teams == nil {
		return nil
	}
	return m.deps.Teams.All()
}

func (m *Model) renderTeams(width, height int) string {
	t := m.theme
	list := m.teamsList()
	var b strings.Builder
	b.WriteString(t.Title.Render("Your teams") + "\n\n")
	if len(list) == 0 {
		b.WriteString(t.Muted.Render("  No teams stored yet.") + "\n")
		b.WriteString(t.Dim.Render("  Import one with : import team") + "\n")
		return b.String()
	}
	for i, team := range list {
		marker := "  "
		if i == m.teamCursor {
			marker = t.Accent.Render("▸ ")
		}
		b.WriteString(marker + t.Fg.Render(team.Name) + " " +
			t.Muted.Render(fmt.Sprintf("(%d)", len(team.Pokemon))) + "\n")
	}
	if m.teamCursor < len(list) {
		b.WriteString("\n" + t.Muted.Render("Preview") + "\n")
		for _, p := range list[m.teamCursor].Pokemon {
			b.WriteString("  " + t.Fg.Render(p.Display()) + " " + t.Muted.Render(p.Item) + "\n")
		}
	}
	b.WriteString("\n" + t.Dim.Render("esc back · : palette to import or export"))
	return b.String()
}

// ---------------------------------------------------------------------------
// Small helpers
// ---------------------------------------------------------------------------

func truncate(s string, n int) string {
	if n <= 1 {
		return s
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
