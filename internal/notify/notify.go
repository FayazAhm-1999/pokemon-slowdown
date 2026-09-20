// Package notify delivers turn notifications through the terminal and, where
// available, the desktop. All delivery is opt-in and rate limited by callers.
package notify

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Mode selects how notifications are delivered.
type Mode string

// Notification modes.
const (
	ModeOff     Mode = "off"
	ModeBell    Mode = "bell"
	ModeOSC     Mode = "osc"
	ModeDesktop Mode = "desktop"
)

// Notifier delivers notifications.
type Notifier struct {
	// Mode is the configured delivery mode.
	Mode Mode
	// Bell rings the terminal bell.
	Bell bool
	// OSC emits an OSC 9 / OSC 777 desktop notification escape sequence, which
	// Ghostty, Kitty, WezTerm and iTerm2 understand.
	OSC bool
	// Desktop uses a platform helper (osascript on macOS).
	Desktop bool
	// Out is where escape sequences are written. Defaults to os.Stderr.
	Out *os.File
}

// New returns a notifier configured for the given mode. Unknown modes fall
// back to the terminal bell.
func New(mode Mode) *Notifier {
	n := &Notifier{Out: os.Stderr, Mode: mode}
	switch mode {
	case ModeOff:
	case ModeBell:
		n.Bell = true
	case ModeDesktop:
		n.Desktop = true
	case ModeOSC:
		n.OSC = true
	default:
		n.Bell = true
	}
	return n
}

// Enabled reports whether any delivery mechanism is active.
func (n *Notifier) Enabled() bool {
	return n != nil && (n.Bell || n.OSC || n.Desktop)
}

// Notify delivers a notification with a title and body.
func (n *Notifier) Notify(title, body string) {
	if n == nil {
		return
	}
	out := n.Out
	if out == nil {
		out = os.Stderr
	}
	if n.Bell {
		fmt.Fprint(out, "\a")
	}
	if n.OSC {
		// OSC 777 is understood by Kitty, WezTerm and Ghostty; OSC 9 by
		// iTerm2 and others. Emit both; terminals ignore what they do not know.
		fmt.Fprintf(out, "\x1b]777;notify;%s;%s\x07", sanitize(title), sanitize(body))
		fmt.Fprintf(out, "\x1b]9;%s\x07", sanitize(title+": "+body))
	}
	if n.Desktop {
		desktopNotify(title, body)
	}
}

// desktopNotify shells out to a platform helper. Arguments are passed as a
// list, never interpolated into a shell string, so a hostile title cannot
// execute anything.
func desktopNotify(title, body string) {
	if runtime.GOOS != "darwin" {
		return
	}
	script := `on run argv
	display notification (item 2 of argv) with title (item 1 of argv)
end run`
	cmd := exec.Command("osascript", "-e", script, sanitize(title), sanitize(body))
	_ = cmd.Run()
}

// sanitize strips control characters so a server-supplied string cannot inject
// terminal escapes through a notification.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}
