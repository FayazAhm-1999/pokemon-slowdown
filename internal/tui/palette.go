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

// promptState is a single-line (or pasted multi-line) text prompt.
type promptState struct {
	open  bool
	label string
	kind  string
	input string
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

	cmds = append(cmds, paletteCommand{
		title: "Challenge a user…",
		run: func(m *Model) tea.Cmd {
			m.prompt = promptState{open: true, label: "Username to challenge with " + def, kind: "challenge"}
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
				m.prompt = promptState{open: true, label: "Paste team text, then enter", kind: "import"}
				return nil
			},
		},
		paletteCommand{
			title: "Spectate a battle…",
			run: func(m *Model) tea.Cmd {
				m.prompt = promptState{open: true, label: "Battle id or replay URL", kind: "spectate"}
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
	case "enter":
		return m.submitPrompt()
	case "backspace":
		if len(m.prompt.input) > 0 {
			r := []rune(m.prompt.input)
			m.prompt.input = string(r[:len(r)-1])
		}
	default:
		if r := printable(key); r != 0 {
			m.prompt.input += string(r)
		} else if key == "space" {
			m.prompt.input += " "
		}
	}
	return nil
}

func (m *Model) submitPrompt() tea.Cmd {
	p := m.prompt
	m.prompt = promptState{}
	input := strings.TrimSpace(p.input)
	if input == "" {
		return nil
	}
	switch p.kind {
	case "challenge":
		if m.deps.Client == nil {
			return nil
		}
		client := m.deps.Client
		format := m.cfg.DefaultFormat
		m.setToast("Challenging " + input + "…")
		return func() tea.Msg { _ = client.Challenge(input, format); return nil }
	case "import":
		team, err := teams.ParseExport(input)
		if err != nil {
			m.setToast("Could not parse team: " + err.Error())
			return nil
		}
		if m.deps.Teams == nil {
			m.setToast("Team storage unavailable.")
			return nil
		}
		team.Name = fmt.Sprintf("Imported %s", team.Pokemon[0].Display())
		if err := m.deps.Teams.Add(team); err != nil {
			m.setToast("Could not save team: " + err.Error())
			return nil
		}
		m.setToast(fmt.Sprintf("Imported %d Pokémon.", len(team.Pokemon)))
		m.screen = screenTeams
		return nil
	case "spectate":
		if m.deps.Client == nil {
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
	return nil
}

func (m *Model) renderPrompt() string {
	t := m.theme
	body := t.Fg.Render(truncate(m.prompt.input, 400)) + t.Primary.Render("▌")
	return t.BoxFocus.Padding(0, 2).Render(
		t.Title.Render(m.prompt.label) + "\n\n" + body + "\n\n" + t.Dim.Render("enter confirm · esc cancel"))
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
