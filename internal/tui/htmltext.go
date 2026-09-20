package tui

import (
	"html"
	"regexp"
	"strings"
)

var (
	// Block-level tags become line breaks before the rest are stripped, so a
	// list of results does not collapse into one run-on line.
	htmlBlockRE = regexp.MustCompile(`(?i)<br\s*/?>|</(p|li|ul|ol|div|tr|td|h[1-6]|blockquote|table)>`)
	htmlTagRE   = regexp.MustCompile(`<[^>]*>`)
)

// looksLikeHTML reports whether a chat message is really a server-rendered
// fragment. Commands such as /data reply with HTML rather than plain text.
func looksLikeHTML(s string) bool {
	if strings.HasPrefix(s, "/raw ") || strings.HasPrefix(s, "/html ") {
		return true
	}
	return strings.Contains(s, "<") && strings.Contains(s, ">")
}

// htmlToText renders a server HTML fragment as readable plain text.
//
// Rich room widgets are out of scope for a terminal client, but a command's
// output should be legible rather than a wall of markup.
func htmlToText(s string) string {
	s = strings.TrimPrefix(s, "/raw ")
	s = strings.TrimPrefix(s, "/html ")
	s = htmlBlockRE.ReplaceAllString(s, "\n")
	s = htmlTagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		// Collapse the runs of spaces and newlines that the markup leaves behind.
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
