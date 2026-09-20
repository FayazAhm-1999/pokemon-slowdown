package tui

import "strings"

// Sanitize removes terminal control sequences from untrusted text before it is
// rendered. Everything that arrives from the server — chat messages, usernames,
// room titles, popups — is untrusted, and in a terminal a stray escape sequence
// can rewrite the screen or worse.
//
// It strips C0 controls (except tab, which callers usually convert to spaces
// first), C1 controls, and whole CSI/OSC/DCS escape sequences.
func Sanitize(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == 0x1b:
			// Skip the whole escape sequence.
			i = skipEscape(runes, i)
		case r == '\t':
			b.WriteByte(' ')
		case r == '\n':
			// Callers that need a single line replace this themselves.
			b.WriteByte('\n')
		case r < 0x20:
			// Drop other C0 controls.
		case r == 0x7f:
			// Drop DEL.
		case r >= 0x80 && r <= 0x9f:
			// Drop C1 controls.
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SanitizeLine is Sanitize with newlines collapsed to spaces, for single-line
// contexts such as usernames, titles and status text.
func SanitizeLine(s string) string {
	out := Sanitize(s)
	out = strings.ReplaceAll(out, "\r", " ")
	out = strings.ReplaceAll(out, "\n", " ")
	return strings.TrimSpace(out)
}

// skipEscape advances past the escape sequence starting at index i and returns
// the index of its last rune.
func skipEscape(runes []rune, i int) int {
	if i+1 >= len(runes) {
		return i
	}
	switch runes[i+1] {
	case '[': // CSI: params then a final byte in @-~
		for j := i + 2; j < len(runes); j++ {
			if runes[j] >= 0x40 && runes[j] <= 0x7e {
				return j
			}
		}
		return len(runes) - 1
	case ']': // OSC: terminated by BEL or ST (ESC \)
		for j := i + 2; j < len(runes); j++ {
			if runes[j] == 0x07 {
				return j
			}
			if runes[j] == 0x1b && j+1 < len(runes) && runes[j+1] == '\\' {
				return j + 1
			}
		}
		return len(runes) - 1
	case 'P', 'X', '^', '_': // DCS, SOS, PM, APC: terminated by ST
		for j := i + 2; j < len(runes); j++ {
			if runes[j] == 0x1b && j+1 < len(runes) && runes[j+1] == '\\' {
				return j + 1
			}
		}
		return len(runes) - 1
	default:
		// Two-character escape (e.g. ESC c).
		return i + 1
	}
}

// clamp bounds n to the range [lo, hi].
func clamp(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
