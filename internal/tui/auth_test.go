package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unnipv/pokemon-slowdown/internal/config"
)

// keyringStub records what would have been stored in the OS keychain, so tests
// never touch the real one.
type keyringStub struct {
	stored  map[string]string
	deleted []string
	failSet bool
}

func newKeyringStub() *keyringStub {
	return &keyringStub{stored: map[string]string{}}
}

func (k *keyringStub) store(user, pass string) error {
	if k.failSet {
		return errStub
	}
	k.stored[user] = pass
	return nil
}

func (k *keyringStub) delete(user string) error {
	delete(k.stored, user)
	k.deleted = append(k.deleted, user)
	return nil
}

type stubError struct{}

func (stubError) Error() string { return "keychain unavailable" }

var errStub = stubError{}

// signInModel returns a model with a stubbed keychain and a throwaway config
// directory, so nothing touches the developer's real setup.
func signInModel(t *testing.T, kr *keyringStub) (*Model, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	m := New(config.Default(), Deps{
		Now:            time.Now,
		StorePassword:  kr.store,
		DeletePassword: kr.delete,
	})
	return m, dir
}

func TestSignInStoresUsernameButNeverThePasswordOnDisk(t *testing.T) {
	kr := newKeyringStub()
	m, dir := signInModel(t, kr)

	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username", value: "coffee_enjoyer"},
		promptField{label: "Password", secret: true, value: "hunter2"},
		promptField{label: "Remember", value: "y"},
	)
	m.submitPrompt()

	if m.cfg.Username != "coffee_enjoyer" {
		t.Errorf("username not saved: %q", m.cfg.Username)
	}
	if !m.cfg.Remember {
		t.Error("remember flag not saved")
	}
	if kr.stored["coffee_enjoyer"] != "hunter2" {
		t.Errorf("password not handed to the keychain: %#v", kr.stored)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "pokemon-slowdown", "config.toml"))
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if strings.Contains(string(raw), "hunter2") {
		t.Fatalf("password leaked into the config file:\n%s", raw)
	}
	if !strings.Contains(string(raw), "coffee_enjoyer") {
		t.Errorf("username missing from the config file:\n%s", raw)
	}
}

func TestSignInWithoutRememberingClearsAnyStoredPassword(t *testing.T) {
	kr := newKeyringStub()
	kr.stored["coffee_enjoyer"] = "old-password"
	m, _ := signInModel(t, kr)

	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username", value: "coffee_enjoyer"},
		promptField{label: "Password", secret: true, value: "hunter2"},
		promptField{label: "Remember", value: "n"},
	)
	m.submitPrompt()

	if m.cfg.Remember {
		t.Error("remember should be false")
	}
	if _, ok := kr.stored["coffee_enjoyer"]; ok {
		t.Error("an existing stored password should be removed when not remembering")
	}
	if len(kr.deleted) == 0 {
		t.Error("expected the stored password to be deleted")
	}
}

func TestSignInRequiresAPassword(t *testing.T) {
	// The server rejects a bare rename, so there is no point pretending a
	// password-less sign-in worked.
	kr := newKeyringStub()
	m, _ := signInModel(t, kr)

	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username", value: "SomeGuest"},
		promptField{label: "Password", secret: true},
		promptField{label: "Remember", value: "y"},
	)
	m.submitPrompt()

	if m.cfg.Username != "" {
		t.Errorf("should not sign in without a password: %q", m.cfg.Username)
	}
	if m.cfg.Remember {
		t.Error("remember should stay off")
	}
	if len(kr.stored) != 0 {
		t.Errorf("nothing should be stored: %#v", kr.stored)
	}
	if m.toast == "" {
		t.Error("the user should be told why nothing happened")
	}
}

func TestSignInWithNoUsernameIsRejected(t *testing.T) {
	kr := newKeyringStub()
	m, _ := signInModel(t, kr)

	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username", value: "   "},
		promptField{label: "Password", secret: true, value: "hunter2"},
		promptField{label: "Remember", value: "y"},
	)
	m.submitPrompt()

	if m.cfg.Username != "" {
		t.Errorf("an empty username should not be saved: %q", m.cfg.Username)
	}
	if len(kr.stored) != 0 {
		t.Error("nothing should be stored without a username")
	}
}

func TestSignInSurvivesAnUnavailableKeychain(t *testing.T) {
	kr := newKeyringStub()
	kr.failSet = true
	m, _ := signInModel(t, kr)

	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username", value: "coffee_enjoyer"},
		promptField{label: "Password", secret: true, value: "hunter2"},
		promptField{label: "Remember", value: "y"},
	)
	m.submitPrompt()

	// A keychain failure must not stop the sign-in, and must not be silent.
	if m.cfg.Username != "coffee_enjoyer" {
		t.Error("username should still be saved")
	}
	if m.toast == "" {
		t.Error("an unavailable keychain should tell the user")
	}
}

func TestSignOutClearsEverything(t *testing.T) {
	kr := newKeyringStub()
	m, dir := signInModel(t, kr)
	m.cfg.Username = "coffee_enjoyer"
	m.cfg.Remember = true
	kr.stored["coffee_enjoyer"] = "hunter2"
	m.username = "coffee_enjoyer"
	m.loggedIn = true

	m.logout()

	if m.cfg.Username != "" || m.cfg.Remember {
		t.Errorf("config not cleared: %#v", m.cfg)
	}
	if _, ok := kr.stored["coffee_enjoyer"]; ok {
		t.Error("stored password should be deleted on sign out")
	}
	if m.loggedIn || m.username != "" {
		t.Errorf("session state not cleared: %q %v", m.username, m.loggedIn)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "pokemon-slowdown", "config.toml"))
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if strings.Contains(string(raw), "coffee_enjoyer") {
		t.Errorf("username should be gone from the config file:\n%s", raw)
	}
}

func TestPasswordIsMaskedOnScreen(t *testing.T) {
	kr := newKeyringStub()
	m, _ := signInModel(t, kr)
	m.width, m.height = 100, 40

	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username", value: "coffee_enjoyer"},
		promptField{label: "Password", secret: true, value: "hunter2"},
	)
	m.prompt.index = 1
	out := m.render()

	if strings.Contains(out, "hunter2") {
		t.Fatal("the password was rendered to the screen")
	}
	if !strings.Contains(out, "•••••••") {
		t.Errorf("expected a masked value: %q", out)
	}
	if !strings.Contains(out, "coffee_enjoyer") {
		t.Error("the username should stay visible")
	}
}

func TestPromptAdvancesThroughFields(t *testing.T) {
	kr := newKeyringStub()
	m, _ := signInModel(t, kr)
	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username"},
		promptField{label: "Password", secret: true},
	)

	for _, r := range "bob" {
		m.handlePromptKey(string(r))
	}
	if got := m.prompt.value(0); got != "bob" {
		t.Fatalf("typing went to the wrong field: %q", got)
	}

	// Enter moves to the next field rather than submitting.
	if cmd := m.handlePromptKey("enter"); cmd != nil {
		t.Fatal("enter on a non-final field must not submit")
	}
	if m.prompt.index != 1 {
		t.Fatalf("index = %d, want 1", m.prompt.index)
	}
	if !m.prompt.open {
		t.Fatal("prompt closed early")
	}

	// Typing now goes to the secret field.
	for _, r := range "pw" {
		m.handlePromptKey(string(r))
	}
	if got := m.prompt.value(1); got != "pw" {
		t.Errorf("secret field = %q", got)
	}
	if m.prompt.value(0) != "bob" {
		t.Errorf("first field changed: %q", m.prompt.value(0))
	}
}

func TestEscapeClosesThePromptWithoutSubmitting(t *testing.T) {
	kr := newKeyringStub()
	m, _ := signInModel(t, kr)
	m.prompt = newPrompt("login", "Sign in",
		promptField{label: "Username", value: "bob"},
		promptField{label: "Password", secret: true, value: "pw"},
	)
	m.prompt.index = 1

	m.handlePromptKey("esc")

	if m.prompt.open {
		t.Error("escape should close the prompt")
	}
	if m.cfg.Username != "" {
		t.Errorf("escape must not sign in: %q", m.cfg.Username)
	}
}
