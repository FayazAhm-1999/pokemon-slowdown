// Package showdown implements the Pokémon Showdown wire protocol:
// connection management, message framing, typed event parsing and outbound
// commands.
//
// The package deliberately has no dependency on the TUI or on the battle
// reducer. It turns bytes into typed events, and typed commands into bytes.
package showdown

import "strings"

// Frame is a run of protocol lines belonging to a single room. An empty RoomID
// means the lines were global (not scoped to a room).
type Frame struct {
	RoomID string
	Lines  []string
}

// SplitFrames splits a raw websocket payload into frames. The server may batch
// several rooms into a single payload; a line beginning with '>' starts a new
// room. Empty lines are ignored, as required by PROTOCOL.md.
func SplitFrames(data string) []Frame {
	var frames []Frame
	var cur Frame
	started := false

	flush := func() {
		if started {
			frames = append(frames, cur)
		}
		cur = Frame{}
		started = false
	}

	for _, line := range strings.Split(data, "\n") {
		// Tolerate CRLF. The protocol is newline-delimited, and a client that
		// chokes on a stray carriage return is a client that breaks on Windows
		// checkouts and on any future server change.
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, ">") {
			flush()
			cur.RoomID = line[1:]
			started = true
			continue
		}
		if line == "" {
			continue
		}
		cur.Lines = append(cur.Lines, line)
		started = true
	}
	flush()
	return frames
}

// splitN splits s on '|' into at most n fields. The final field keeps any
// remaining separators, which is required for chat messages and any payload
// that may legitimately contain '|'.
func splitN(s string, n int) []string {
	if n <= 0 {
		return nil
	}
	parts := make([]string, 0, n)
	for len(parts) < n-1 {
		i := strings.IndexByte(s, '|')
		if i < 0 {
			break
		}
		parts = append(parts, s[:i])
		s = s[i+1:]
	}
	return append(parts, s)
}

// Tags holds the optional trailing tags on a protocol line, for example
// "[from] item: Life Orb" or a bare "[still]".
type Tags map[string]string

// Has reports whether a bare or valued tag is present.
func (t Tags) Has(key string) bool { _, ok := t[key]; return ok }

// Get returns the value of a tag, or "" for a bare tag or a missing tag.
func (t Tags) Get(key string) string { return t[key] }

// isTagArg reports whether an argument is a protocol tag such as "[from]",
// "[miss]" or "[from] item: Life Orb". Tag keys are lowercase identifiers, so
// values like "[Gen 9] Random Battle" are correctly left alone.
func isTagArg(a string) bool {
	if len(a) < 3 || a[0] != '[' {
		return false
	}
	end := strings.IndexByte(a, ']')
	if end < 2 {
		return false
	}
	for i := 1; i < end; i++ {
		c := a[i]
		if (c < 'a' || c > 'z') && c != '-' {
			return false
		}
	}
	return true
}

// parseTags extracts every "[key]" / "[key] value" argument into a Tags map.
func parseTags(args []string) Tags {
	tags := Tags{}
	for _, a := range args {
		if !isTagArg(a) {
			continue
		}
		end := strings.IndexByte(a, ']')
		tags[a[1:end]] = strings.TrimSpace(a[end+1:])
	}
	return tags
}

// splitArgs splits a protocol line into its arguments and trailing tags. The
// leading empty field produced by the leading '|' is dropped.
func splitArgs(line string) ([]string, Tags) {
	parts := strings.Split(line, "|")
	if len(parts) > 0 && parts[0] == "" {
		parts = parts[1:]
	}
	args := make([]string, 0, len(parts))
	for _, p := range parts {
		if isTagArg(p) {
			continue
		}
		args = append(args, p)
	}
	return args, parseTags(parts)
}

// ToID normalises a name to a Pokémon Showdown identifier: lowercase with all
// non-alphanumeric characters removed.
func ToID(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		}
	}
	return b.String()
}

// User is an entry from a room's |users| message.
type User struct {
	Name   string
	Rank   string
	Status string
}

// Challenge describes an incoming or outgoing challenge.
type Challenge struct {
	To     string
	Format string
}

// Format is one entry from the server's |formats| message. The server sends a
// display name plus a hex flag bitmask; the ID is derived from the name.
type Format struct {
	// ID is the format identifier, e.g. "gen9randombattle".
	ID string
	// Name is the human-readable name, e.g. "[Gen 9] Random Battle".
	Name string
	// Section is the catalogue section, e.g. "S/V Singles".
	Section string
	// Random marks formats played with preset teams, such as Random Battle.
	Random bool
	// Searchable marks formats that can be queued on the ladder.
	Searchable bool
	// Challengeable marks formats that can be played via challenge.
	Challengeable bool
	// Tournament marks formats usable in tournaments.
	Tournament bool
}
