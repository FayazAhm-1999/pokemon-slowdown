package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sahilm/fuzzy"

	"github.com/unnipv/pokemon-slowdown/internal/showdown"
	"github.com/unnipv/pokemon-slowdown/internal/sprites"
	"github.com/unnipv/pokemon-slowdown/internal/teams"
)

// paletteState is the fuzzy command palette.
type paletteState struct {
	open   bool
	filter string
	cursor int
}

// promptField is one input in a prompt. Secret fields are masked on screen and
// are never logged, written to the config file, or echoed anywhere else.
type promptField struct {
	label  string
	secret bool
	value  string
}

// promptState is a small multi-field form.
type promptState struct {
	open   bool
	title  string
	kind   string
	fields []promptField
	index  int
}

// newPrompt builds an open prompt with the given fields.
func newPrompt(kind, title string, fields ...promptField) promptState {
	return promptState{open: true, kind: kind, title: title, fields: fields}
}

func (p *promptState) current() *promptField {
	if p.index < 0 || p.index >= len(p.fields) {
		return nil
	}
	return &p.fields[p.index]
}

// value returns a field's contents, or "" when out of range.
func (p promptState) value(i int) string {
	if i < 0 || i >= len(p.fields) {
		return ""
	}
	return p.fields[i].value
}

// masked renders a value, hiding it entirely when the field is secret.
func (f promptField) masked() string {
	if !f.secret {
		return f.value
	}
	return strings.Repeat("•", len([]rune(f.value)))
}

// paletteCommand is one palette entry.
type paletteCommand struct {
	title string
	run   func(*Model) tea.Cmd
}

func (m *Model) openPalette() {
	m.palette = paletteState{open: true}
}

// commands builds the palette entries for the current application state.
func (m *Model) commands() []paletteCommand {
	var cmds []paletteCommand

	def := FormatName(m.cfg.DefaultFormat)
	cmds = append(cmds, paletteCommand{
		title: "Queue " + def,
		run: func(m *Model) tea.Cmd {
			return m.queueFormat(showdown.Format{ID: m.cfg.DefaultFormat, Random: true, Searchable: true, Challengeable: true})
		},
	})
	cmds = append(cmds, paletteCommand{
		title: "Find a battle (format picker)",
		run: func(m *Model) tea.Cmd {
			m.screen = screenLobby
			return nil
		},
	})

	if len(m.searching) > 0 {
		cmds = append(cmds, paletteCommand{
			title: "Cancel search",
			run: func(m *Model) tea.Cmd {
				if m.deps.Client == nil {
					return nil
				}
				client := m.deps.Client
				m.setToast("Search cancelled.")
				return func() tea.Msg { _ = client.CancelSearch(); return nil }
			},
		})
	}

	if m.loggedIn {
		cmds = append(cmds, paletteCommand{
			title: "Sign out of " + m.username,
			run:   func(m *Model) tea.Cmd { return m.logout() },
		})
	} else {
		cmds = append(cmds, paletteCommand{
			title: "Sign in to a registered account…",
			run: func(m *Model) tea.Cmd {
				m.prompt = newPrompt("login", "Sign in to Pokémon Showdown",
					promptField{label: "Username", value: m.cfg.Username},
					promptField{label: "Password", secret: true},
					promptField{label: "Remember", value: "n"},
				)
				return nil
			},
		})
	}

	cmds = append(cmds, paletteCommand{
		title: "Challenge a user…",
		run: func(m *Model) tea.Cmd {
			m.prompt = newPrompt("challenge", "Challenge a user", promptField{label: "Username"})
			return nil
		},
	})

	users := make([]string, 0, len(m.challengesFrom))
	for u := range m.challengesFrom {
		users = append(users, u)
	}
	sort.Strings(users)
	for _, u := range users {
		user := u
		format := m.challengesFrom[user]
		cmds = append(cmds, paletteCommand{
			title: "Accept challenge from " + user + " (" + FormatName(format) + ")",
			run: func(m *Model) tea.Cmd {
				if m.deps.Client == nil {
					return nil
				}
				client := m.deps.Client
				m.setToast("Accepted challenge from " + user)
				return func() tea.Msg { _ = client.AcceptChallenge(user); return nil }
			},
		})
		cmds = append(cmds, paletteCommand{
			title: "Reject challenge from " + user,
			run: func(m *Model) tea.Cmd {
				if m.deps.Client == nil {
					return nil
				}
				client := m.deps.Client
				return func() tea.Msg { _ = client.RejectChallenge(user); return nil }
			},
		})
	}

	for _, room := range m.order {
		room := room
		bv := m.battles[room]
		title := room
		if bv != nil {
			title = bv.title()
		}
		cmds = append(cmds, paletteCommand{
			title: "Open battle: " + title,
			run: func(m *Model) tea.Cmd {
				m.openBattle(room)
				return nil
			},
		})
	}

	cmds = append(cmds,
		paletteCommand{
			title: "Teams",
			run:   func(m *Model) tea.Cmd { m.screen = screenTeams; return nil },
		},
		paletteCommand{
			title: "Import team from clipboard text…",
			run: func(m *Model) tea.Cmd {
				m.prompt = newPrompt("import", "Import a team", promptField{label: "Team text"})
				return nil
			},
		},
		paletteCommand{
			title: "Spectate a battle…",
			run: func(m *Model) tea.Cmd {
				m.prompt = newPrompt("spectate", "Spectate a battle", promptField{label: "Battle id"})
				return nil
			},
		},
	)

	spriteLabel := "Toggle sprites (currently " + m.cfg.Sprites.Mode + ")"
	cmds = append(cmds, paletteCommand{
		title: spriteLabel,
		run: func(m *Model) tea.Cmd {
			if m.cfg.Sprites.Mode == "none" {
				m.cfg.Sprites.Mode = "blocks"
			} else {
				m.cfg.Sprites.Mode = "none"
			}
			m.rebuildRenderer()
			m.setToast("Sprites: " + m.cfg.Sprites.Mode)
			return nil
		},
	})
	cmds = append(cmds, paletteCommand{
		title: fmt.Sprintf("Toggle animation (currently %v)", m.cfg.Sprites.Animate),
		run: func(m *Model) tea.Cmd {
			m.cfg.Sprites.Animate = !m.cfg.Sprites.Animate
			m.setToast(fmt.Sprintf("Animation: %v", m.cfg.Sprites.Animate))
			return nil
		},
	})
	cmds = append(cmds, paletteCommand{
		title: "Next theme",
		run: func(m *Model) tea.Cmd {
			m.cfg.Theme = nextTheme(m.cfg.Theme)
			m.theme = LoadTheme(m.cfg.Theme)
			m.setToast("Theme: " + m.cfg.Theme)
			return nil
		},
	})
	cmds = append(cmds, paletteCommand{
		title: "Save settings",
		run: func(m *Model) tea.Cmd {
			if err := m.cfg.Save(""); err != nil {
				m.setToast("Could not save settings: " + err.Error())
			} else {
				m.setToast("Settings saved.")
			}
			return nil
		},
	})
	if bv := m.activeBattle(); bv != nil && !bv.state().Ended {
		cmds = append(cmds, paletteCommand{
			title: "Forfeit this battle",
			run: func(m *Model) tea.Cmd {
				m.confirm = "Forfeit this battle?"
				m.confirmAction = "forfeit"
				return nil
			},
		})
	}
	cmds = append(cmds,
		paletteCommand{title: "Keyboard help", run: func(m *Model) tea.Cmd { m.helpOpen = true; return nil }},
		paletteCommand{title: "Quit", run: func(m *Model) tea.Cmd { return tea.Quit }},
	)
	return cmds
}

func (m *Model) filteredCommands() []paletteCommand {
	all := m.commands()
	if m.palette.filter == "" {
		return all
	}
	titles := make([]string, len(all))
	for i, c := range all {
		titles[i] = c.title
	}
	matches := fuzzy.Find(m.palette.filter, titles)
	out := make([]paletteCommand, 0, len(matches))
	for _, mt := range matches {
		out = append(out, all[mt.Index])
	}
	return out
}

func (m *Model) handlePaletteKey(key string) tea.Cmd {
	cmds := m.filteredCommands()
	switch key {
	case "esc":
		m.palette.open = false
	case "up", "ctrl+p":
		m.palette.cursor = clamp(m.palette.cursor-1, 0, max(0, len(cmds)-1))
	case "down", "ctrl+n":
		m.palette.cursor = clamp(m.palette.cursor+1, 0, max(0, len(cmds)-1))
	case "enter":
		if m.palette.cursor < len(cmds) {
			cmd := cmds[m.palette.cursor]
			m.palette.open = false
			return cmd.run(m)
		}
	case "backspace":
		if m.palette.filter != "" {
			m.palette.filter = m.palette.filter[:len(m.palette.filter)-1]
			m.palette.cursor = 0
		}
	default:
		if r := printable(key); r != 0 {
			m.palette.filter += string(r)
			m.palette.cursor = 0
		} else if key == "space" {
			m.palette.filter += " "
			m.palette.cursor = 0
		}
	}
	return nil
}

func (m *Model) renderPalette() string {
	t := m.theme
	cmds := m.filteredCommands()
	rows := 10
	if len(cmds) < rows {
		rows = len(cmds)
	}
	var b strings.Builder
	filter := m.palette.filter
	if filter == "" {
		filter = t.Dim.Render("type a command…")
	} else {
		filter = t.Fg.Render(filter) + t.Primary.Render("▌")
	}
	b.WriteString(t.Title.Render("Commands") + "\n")
	b.WriteString(t.Muted.Render(": ") + filter + "\n\n")
	if rows == 0 {
		b.WriteString(t.Muted.Render("  no matching command") + "\n")
	}
	start := 0
	if m.palette.cursor >= rows {
		start = m.palette.cursor - rows + 1
	}
	for i := start; i < len(cmds) && i < start+rows; i++ {
		if i == m.palette.cursor {
			b.WriteString(t.Selected.Render("▸ "+truncate(cmds[i].title, 60)) + "\n")
		} else {
			b.WriteString("  " + t.Fg.Render(truncate(cmds[i].title, 60)) + "\n")
		}
	}
	b.WriteString("\n" + t.Dim.Render("↑/↓ choose · enter run · esc close"))
	return t.BoxFocus.Padding(0, 2).Render(b.String())
}

// ---------------------------------------------------------------------------
// Prompt
// ---------------------------------------------------------------------------

func (m *Model) handlePromptKey(key string) tea.Cmd {
	switch key {
	case "esc":
		m.prompt = promptState{}
		return nil
	case "enter":
		if m.prompt.index < len(m.prompt.fields)-1 {
			m.prompt.index++
			return nil
		}
		return m.submitPrompt()
	case "backspace":
		if f := m.prompt.current(); f != nil && len(f.value) > 0 {
			r := []rune(f.value)
			f.value = string(r[:len(r)-1])
		}
	case "space":
		if f := m.prompt.current(); f != nil {
			f.value += " "
		}
	default:
		if f := m.prompt.current(); f != nil {
			if r := printable(key); r != 0 {
				f.value += string(r)
			}
		}
	}
	return nil
}

func (m *Model) submitPrompt() tea.Cmd {
	p := m.prompt
	m.prompt = promptState{}
	switch p.kind {
	case "login":
		return m.submitLogin(p)
	case "challenge":
		return m.submitChallenge(strings.TrimSpace(p.value(0)))
	case "import":
		return m.submitImport(p.value(0))
	case "spectate":
		return m.submitSpectate(strings.TrimSpace(p.value(0)))
	}
	return nil
}

// submitLogin stores the credentials and reconnects so the server issues a
// fresh challenge and completes the handshake.
func (m *Model) submitLogin(p promptState) tea.Cmd {
	username := strings.TrimSpace(p.value(0))
	password := p.value(1)
	remember := strings.EqualFold(strings.TrimSpace(p.value(2)), "y")

	if username == "" {
		m.setToast("Enter a username.")
		return nil
	}
	if password == "" {
		// The server will not accept a name without an assertion, so there is
		// nothing useful to do without a password.
		m.setToast("Enter your password. (Guest names are assigned by the server.)")
		return nil
	}

	// Persist the choice first, so it survives even if we are offline. The
	// password itself is never written to the config file.
	m.cfg.Username = username
	m.cfg.Remember = remember && password != ""
	if err := m.cfg.Save(""); err != nil {
		m.setToast("Could not save settings: " + err.Error())
	}

	if m.cfg.Remember {
		if err := m.deps.StorePassword(username, password); err != nil {
			m.setToast("Could not store the password in your keychain; you will need to sign in again next time.")
		}
	} else {
		_ = m.deps.DeletePassword(username)
	}

	if m.deps.Client == nil {
		return nil
	}

	m.deps.Client.Login(showdown.Credentials{Username: username, Password: password})
	if password == "" {
		m.setToast("Signing in as guest " + username + "…")
	} else {
		m.setToast("Signing in as " + username + "…")
	}
	return nil
}

func (m *Model) submitChallenge(user string) tea.Cmd {
	if user == "" || m.deps.Client == nil {
		return nil
	}
	client := m.deps.Client
	format := m.cfg.DefaultFormat
	m.setToast("Challenging " + user + "…")
	return func() tea.Msg { _ = client.Challenge(user, format); return nil }
}

func (m *Model) submitImport(text string) tea.Cmd {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	team, err := teams.ParseExport(text)
	if err != nil {
		m.setToast("Could not parse team: " + err.Error())
		return nil
	}
	if m.deps.Teams == nil {
		m.setToast("Team storage unavailable.")
		return nil
	}
	team.Name = "Imported " + team.Pokemon[0].Display()
	if err := m.deps.Teams.Add(team); err != nil {
		m.setToast("Could not save team: " + err.Error())
		return nil
	}
	m.setToast(fmt.Sprintf("Imported %d Pokémon.", len(team.Pokemon)))
	m.screen = screenTeams
	return nil
}

func (m *Model) submitSpectate(input string) tea.Cmd {
	if input == "" || m.deps.Client == nil {
		return nil
	}
	id := battleIDFromInput(input)
	if id == "" {
		m.setToast("Could not read a battle id from that.")
		return nil
	}
	client := m.deps.Client
	m.setToast("Joining " + id + "…")
	return func() tea.Msg { _ = client.JoinRoom(id); return nil }
}

// logout clears stored credentials, including the keychain entry.
func (m *Model) logout() tea.Cmd {
	user := m.cfg.Username
	m.cfg.Username = ""
	m.cfg.Remember = false
	if err := m.cfg.Save(""); err != nil {
		m.setToast("Could not save settings: " + err.Error())
	}
	if user != "" {
		_ = m.deps.DeletePassword(user)
	}
	if m.deps.Client != nil {
		m.deps.Client.SetCredentials(showdown.Credentials{})
		_ = m.deps.Client.Logout()
		m.deps.Client.Reconnect()
	}
	m.username = ""
	m.loggedIn = false
	m.setToast("Signed out.")
	return nil
}

func (m *Model) renderPrompt() string {
	t := m.theme
	var b strings.Builder
	b.WriteString(t.Title.Render(m.prompt.title) + "\n\n")

	for i, f := range m.prompt.fields {
		label := padRight(f.label, 12)
		if i == m.prompt.index {
			b.WriteString("  " + t.Muted.Render(label) +
				t.Fg.Render(truncate(f.masked(), 40)) + t.Primary.Render("▌") + "\n")
			continue
		}
		// Completed fields stay visible but dimmed. Secret values stay masked
		// even after they are entered.
		b.WriteString("  " + t.Dim.Render(label+truncate(f.masked(), 40)) + "\n")
	}

	hint := "enter next · esc cancel"
	if m.prompt.index == len(m.prompt.fields)-1 {
		hint = "enter confirm · esc cancel"
	}
	b.WriteString("\n" + t.Dim.Render(hint))
	return t.BoxFocus.Padding(0, 2).Render(b.String())
}

// battleIDFromInput extracts a battle room id from an id or a replay URL.
func battleIDFromInput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.Index(s, "replay.pokemonshowdown.com/"); i >= 0 {
		id := s[i+len("replay.pokemonshowdown.com/"):]
		id = strings.Trim(id, "/")
		if !strings.HasPrefix(id, "battle-") {
			id = "battle-" + id
		}
		return id
	}
	if strings.HasPrefix(s, "battle-") {
		return s
	}
	return ""
}

func nextTheme(current string) string {
	for i, n := range ThemeNames {
		if n == current {
			return ThemeNames[(i+1)%len(ThemeNames)]
		}
	}
	return ThemeNames[0]
}

// rebuildRenderer swaps the sprite backend after a settings change.
func (m *Model) rebuildRenderer() {
	if m.deps.Sprites == nil {
		return
	}
	r, err := sprites.NewRenderer(sprites.ParseMode(m.cfg.Sprites.Mode), nil)
	if err != nil {
		return
	}
	m.deps.Renderer = r
	for _, bv := range m.battles {
		bv.spriteRendered = map[string]string{}
	}
}

// FormatName renders a human-readable name for a format id, preferring the
// curated table and falling back to a heuristic prettifier so newly added
// formats still look reasonable.
func FormatName(id string) string {
	if n, ok := formatNames[id]; ok {
		return n
	}
	return prettifyFormat(id)
}

var formatNames = map[string]string{
	"gen9randombattle":            "Gen 9 Random Battle",
	"gen9randomdoublesbattle":     "Gen 9 Random Doubles",
	"gen9ou":                      "Gen 9 OU",
	"gen9ubers":                   "Gen 9 Ubers",
	"gen9uu":                      "Gen 9 UU",
	"gen9ru":                      "Gen 9 RU",
	"gen9nu":                      "Gen 9 NU",
	"gen9pu":                      "Gen 9 PU",
	"gen9lc":                      "Gen 9 Little Cup",
	"gen9monotype":                "Gen 9 Monotype",
	"gen9anythinggoes":            "Gen 9 Anything Goes",
	"gen9doublesou":               "Gen 9 Doubles OU",
	"gen9doublesuu":               "Gen 9 Doubles UU",
	"gen9vgc2024regg":             "Gen 9 VGC 2024 Reg G",
	"gen9vgc2025regg":             "Gen 9 VGC 2025 Reg G",
	"gen9nationaldex":             "Gen 9 National Dex",
	"gen9nationaldexubers":        "Gen 9 National Dex Ubers",
	"gen9balancedhackmons":        "Gen 9 Balanced Hackmons",
	"gen9almostanyability":        "Gen 9 Almost Any Ability",
	"gen9monorandom":              "Gen 9 Monotype Random",
	"gen8randombattle":            "Gen 8 Random Battle",
	"gen8ou":                      "Gen 8 OU",
	"gen8ubers":                   "Gen 8 Ubers",
	"gen8doublesou":               "Gen 8 Doubles OU",
	"gen7randombattle":            "Gen 7 Random Battle",
	"gen7ou":                      "Gen 7 OU",
	"gen6randombattle":            "Gen 6 Random Battle",
	"gen6ou":                      "Gen 6 OU",
	"gen5randombattle":            "Gen 5 Random Battle",
	"gen4randombattle":            "Gen 4 Random Battle",
	"gen3randombattle":            "Gen 3 Random Battle",
	"gen2randombattle":            "Gen 2 Random Battle",
	"gen1randombattle":            "Gen 1 Random Battle",
	"gen1ou":                      "Gen 1 OU",
	"gen9challengecup":            "Gen 9 Challenge Cup",
	"gen9hackmonscup":             "Gen 9 Hackmons Cup",
	"gen9customgame":              "Gen 9 Custom Game",
	"gen9doublescustomgame":       "Gen 9 Doubles Custom Game",
	"gen9metronomebattle":         "Gen 9 Metronome Battle",
	"gen9multi":                   "Gen 9 Multi Battle",
	"gen9freeforall":              "Gen 9 Free-For-All",
	"gen9randombattleblitz":       "Gen 9 Random Battle (Blitz)",
	"gen9oublitz":                 "Gen 9 OU (Blitz)",
	"gen9nationaldexrandombattle": "Gen 9 National Dex Random",
}

// prettifyFormat turns a format id into something readable when it is not in
// the curated table.
func prettifyFormat(id string) string {
	s := id
	gen := ""
	if strings.HasPrefix(s, "gen") {
		i := 3
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i > 3 {
			gen = "Gen " + s[3:i] + " "
			s = s[i:]
		}
	}
	words := []struct{ token, word string }{
		{"randombattle", "Random Battle"}, {"randomdoublesbattle", "Random Doubles"},
		{"doublesou", "Doubles OU"}, {"doublesuu", "Doubles UU"}, {"doublesubers", "Doubles Ubers"},
		{"doublescustomgame", "Doubles Custom Game"},
		{"nationaldex", "National Dex"}, {"anythinggoes", "Anything Goes"},
		{"balancedhackmons", "Balanced Hackmons"}, {"almostanyability", "Almost Any Ability"},
		{"monotype", "Monotype"}, {"monorandom", "Monotype Random"},
		{"challengecup", "Challenge Cup"}, {"hackmonscup", "Hackmons Cup"},
		{"customgame", "Custom Game"}, {"metronomebattle", "Metronome Battle"},
		{"littlecup", "Little Cup"}, {"freeforall", "Free-For-All"},
		{"randombattleblitz", "Random Battle (Blitz)"},
		{"vgc", "VGC"}, {"ubers", "Ubers"}, {"oublitz", "OU (Blitz)"},
		{"ou", "OU"}, {"uu", "UU"}, {"ru", "RU"}, {"nu", "NU"}, {"pu", "PU"}, {"lc", "LC"},
		{"multi", "Multi Battle"}, {"blitz", "Blitz"},
	}
	out := s
	for _, w := range words {
		if out == w.token {
			return gen + w.word
		}
		if strings.HasSuffix(out, w.token) && len(out) > len(w.token) {
			prefix := out[:len(out)-len(w.token)]
			if prefix == "vgc" {
				prefix = "VGC "
			}
			return gen + strings.ToUpper(prefix) + " " + w.word
		}
	}
	if out == "" {
		return gen
	}
	return gen + out
}
