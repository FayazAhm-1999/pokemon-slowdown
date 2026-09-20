package showdown

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestLiveServerConnect is an opt-in integration test against the real Pokémon
// Showdown service. It verifies the parts that a mock server cannot: that the
// endpoint, framing, login flow and format catalogue still work as documented.
//
//	SLOWDOWN_LIVE=1 go test ./internal/showdown/ -run TestLiveServerConnect -v
func TestLiveServerConnect(t *testing.T) {
	if os.Getenv("SLOWDOWN_LIVE") == "" {
		t.Skip("set SLOWDOWN_LIVE=1 to run against the live Pokémon Showdown server")
	}

	guest := fmt.Sprintf("slowdown%d", time.Now().UnixNano()%1_000_000)
	c := NewClient(DefaultURL)
	c.SetCredentials(Credentials{Username: guest})
	c.Start()
	defer c.Close()

	var (
		gotChallstr bool
		gotFormats  bool
		gotUser     bool
		formats     []Format
	)

	deadline := time.After(45 * time.Second)
	for !(gotChallstr && gotFormats) {
		select {
		case ev, ok := <-c.Events():
			if !ok {
				t.Fatal("event channel closed early")
			}
			switch e := ev.(type) {
			case ChallStr:
				gotChallstr = e.Challstr != ""
			case FormatsUpdated:
				gotFormats = true
				formats = e.Formats
			case UpdateUser:
				gotUser = true
			}
		case <-deadline:
			t.Fatalf("timed out (challstr=%v formats=%v user=%v)", gotChallstr, gotFormats, gotUser)
		}
	}

	if len(formats) < 50 {
		t.Errorf("only %d formats returned; expected a full catalogue", len(formats))
	}

	var foundRandom bool
	for _, f := range formats {
		if f.ID == "gen9randombattle" {
			foundRandom = true
			if !f.Random {
				t.Error("gen9randombattle should be flagged as a random format")
			}
			if !f.Searchable {
				t.Error("gen9randombattle should be searchable")
			}
		}
	}
	if !foundRandom {
		t.Error("gen9randombattle missing from the live format list")
	}

	// A guest rename should be acknowledged with |updateuser|.
	if !gotUser {
		if waitFor(t, 15*time.Second, func() bool { return gotUser }) {
			t.Log("guest rename acknowledged late")
		} else {
			t.Error("guest rename was never acknowledged by the server")
		}
	}

	t.Logf("connected: %d formats, guest name %q, logged in=%v", len(formats), guest, gotUser)
}
