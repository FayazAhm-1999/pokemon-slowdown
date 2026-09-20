package showdown

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestLiveSignInClaimsAName exercises the mid-session sign-in path against the
// real server: the client must reconnect to obtain a fresh challenge, then
// complete the handshake and adopt the name.
//
// Note: /api/login with an unregistered name returns an assertion for that name,
// so an unused name is claimed rather than rejected. Rejection only happens for
// a *registered* name with a wrong password, which is covered offline in
// TestLoginFailureIsReported rather than by guessing someone's account here.
//
//	SLOWDOWN_LIVE=1 go test ./internal/showdown/ -run TestLiveSignIn -v
func TestLiveSignInClaimsAName(t *testing.T) {
	if os.Getenv("SLOWDOWN_LIVE") == "" {
		t.Skip("set SLOWDOWN_LIVE=1 to run against the live Pokémon Showdown server")
	}

	c := NewClient(DefaultURL)
	c.BackoffMin = 100 * time.Millisecond
	c.BackoffMax = 2 * time.Second
	c.Start()
	defer c.Close()

	var challenges int
	var reconnected bool
	var authErr string
	var signedInAs string

	deadline := time.After(30 * time.Second)
	for {
		select {
		case ev, ok := <-c.Events():
			if !ok {
				t.Fatal("event channel closed")
			}
			switch e := ev.(type) {
			case ChallStr:
				challenges++
			case UpdateUser:
				t.Logf("initial identity: %q (named=%v)", e.Name, e.Named)
				goto ready
			}
		case <-deadline:
			t.Fatal("timed out waiting for the initial handshake")
		}
	}
ready:

	// Under the 18 character limit the server enforces.
	name := fmt.Sprintf("sdtest%d", time.Now().UnixNano()%1000000)
	t.Logf("signing in as %q", name)
	c.Login(Credentials{Username: name, Password: "a-password-for-an-unregistered-name"})

	deadline = time.After(30 * time.Second)
	for signedInAs == "" && authErr == "" {
		select {
		case ev, ok := <-c.Events():
			if !ok {
				t.Fatal("event channel closed")
			}
			switch e := ev.(type) {
			case Connected:
				if e.Reconnected {
					reconnected = true
				}
			case ChallStr:
				challenges++
			case UpdateUser:
				if ToID(e.Name) == ToID(name) {
					signedInAs = e.Name
					t.Logf("signed in as %q (named=%v)", e.Name, e.Named)
				}
			case AuthFailed:
				authErr = e.Err.Error()
			case NameTaken:
				authErr = "name taken: " + e.Message
			case Popup:
				authErr = "popup: " + e.Message
			}
		case <-deadline:
			t.Fatalf("timed out: reconnected=%v challenges=%d", reconnected, challenges)
		}
	}

	if !reconnected {
		t.Error("signing in must reconnect to obtain a fresh challenge")
	}
	if challenges < 2 {
		t.Errorf("expected a fresh challenge after reconnecting, saw %d total", challenges)
	}
	if authErr != "" {
		t.Fatalf("sign-in failed: %s", authErr)
	}
	if signedInAs == "" {
		t.Fatal("never adopted the requested name")
	}
	t.Log("sign-in completed through the documented assertion flow")
}
