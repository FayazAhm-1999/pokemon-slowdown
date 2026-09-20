package showdown

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// wsServer is a scriptable mock Pokémon Showdown server.
type wsServer struct {
	t      *testing.T
	srv    *httptest.Server
	mu     sync.Mutex
	onOpen func(conn *websocket.Conn)
	conns  int
	connCh chan *websocket.Conn
}

func newWSServer(t *testing.T, onOpen func(conn *websocket.Conn)) *wsServer {
	t.Helper()
	s := &wsServer{t: t, onOpen: onOpen, connCh: make(chan *websocket.Conn, 8)}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns++
		s.mu.Unlock()
		select {
		case s.connCh <- conn:
		default:
		}
		if s.onOpen != nil {
			s.onOpen(conn)
		}
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *wsServer) url() string { return "ws" + strings.TrimPrefix(s.srv.URL, "http") }

func (s *wsServer) connectionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.conns
}

// readCommand reads one client message and asserts it is well formed. The live
// server silently drops any message without a room separator, so a bare command
// would look like a passing test while doing nothing in production.
func readCommand(t *testing.T, conn *websocket.Conn, timeout time.Duration) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		return ""
	}
	msg := string(data)
	if !strings.Contains(msg, "|") {
		t.Errorf("server would drop this message: no room separator: %q", msg)
	}
	return msg
}

// waitFor polls until fn is true or the deadline passes.
func waitFor(t *testing.T, timeout time.Duration, fn func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fn()
}

func collect(t *testing.T, c *Client, until func(Event) bool, timeout time.Duration) []Event {
	t.Helper()
	var out []Event
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-c.Events():
			if !ok {
				return out
			}
			out = append(out, ev)
			if until(ev) {
				return out
			}
		case <-deadline:
			return out
		}
	}
}

func TestClientParsesFramesOverTheWire(t *testing.T) {
	done := make(chan struct{})
	srv := newWSServer(t, func(conn *websocket.Conn) {
		ctx := context.Background()
		_ = conn.Write(ctx, websocket.MessageText, []byte("|challstr|2|abc|def"))
		_ = conn.Write(ctx, websocket.MessageText, []byte(">battle-gen9randombattle-1\n|player|p1|Alice|1|1500\n|turn|1"))
		close(done)
	})

	c := NewClient(srv.url())
	c.Start()
	defer c.Close()

	evs := collect(t, c, func(ev Event) bool {
		_, ok := ev.(BattleTurn)
		return ok
	}, 5*time.Second)

	var sawChall, sawPlayer, sawTurn bool
	for _, ev := range evs {
		switch e := ev.(type) {
		case ChallStr:
			sawChall = e.Challstr == "2|abc|def"
		case BattlePlayer:
			sawPlayer = e.RoomID == "battle-gen9randombattle-1" && e.Name == "Alice"
		case BattleTurn:
			sawTurn = e.Turn == 1
		}
	}
	if !sawChall || !sawPlayer || !sawTurn {
		t.Fatalf("chall=%v player=%v turn=%v (%#v)", sawChall, sawPlayer, sawTurn, evs)
	}
}

func TestGuestLoginSendsTrn(t *testing.T) {
	connCh := make(chan *websocket.Conn, 1)
	srv := newWSServer(t, func(conn *websocket.Conn) {
		_ = conn.Write(context.Background(), websocket.MessageText, []byte("|challstr|2|xyz"))
		connCh <- conn
	})

	c := NewClient(srv.url())
	c.SetCredentials(Credentials{Username: "slowpoke"})
	c.Start()
	defer c.Close()

	var conn *websocket.Conn
	select {
	case conn = <-connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("server never accepted a connection")
	}

	if got := readCommand(t, conn, 5*time.Second); got != "|/trn slowpoke" {
		t.Fatalf("guest login command = %q, want |/trn slowpoke", got)
	}
}

func TestFetchAssertionStripsBracketPrefix(t *testing.T) {
	var gotForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		gotForm = r.PostForm
		_, _ = w.Write([]byte(`]{"assertion":"ASSERT-123","curuser":{"loggedin":true,"userid":"alice"},"actionsuccess":true}`))
	}))
	defer srv.Close()

	assertion, err := fetchAssertion(context.Background(), srv.URL, "2|2023-09-01|abc|1234", "alice", "hunter2", srv.Client())
	if err != nil {
		t.Fatalf("fetchAssertion: %v", err)
	}
	if assertion != "ASSERT-123" {
		t.Fatalf("assertion = %q", assertion)
	}
	if gotForm.Get("name") != "alice" || gotForm.Get("pass") != "hunter2" {
		t.Fatalf("unexpected form: %#v", gotForm)
	}
	// Real challstr values contain pipes and must survive form encoding.
	if got := gotForm.Get("challstr"); got != "2|2023-09-01|abc|1234" {
		t.Fatalf("challstr = %q", got)
	}
}

func TestLoginFailureReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`]{"actionsuccess":false,"curuser":{"loggedin":false}}`))
	}))
	defer srv.Close()

	assertion, err := fetchAssertion(context.Background(), srv.URL, "2|x", "alice", "wrong", srv.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assertion != "" {
		t.Fatalf("expected empty assertion, got %q", assertion)
	}
}

func TestClientReconnectsAndRejoins(t *testing.T) {
	var joined []string
	var mu sync.Mutex
	var connN int
	srv := newWSServer(t, func(conn *websocket.Conn) {
		mu.Lock()
		connN++
		n := connN
		mu.Unlock()
		if n == 1 {
			// First connection: close it immediately to force a reconnect.
			_ = conn.Close(websocket.StatusInternalError, "boom")
			return
		}
		// Later connections: record what the client rejoins with.
		for {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_, data, err := conn.Read(ctx)
			cancel()
			if err != nil {
				return
			}
			mu.Lock()
			joined = append(joined, string(data))
			mu.Unlock()
		}
	})

	c := NewClient(srv.url())
	c.BackoffMin = 10 * time.Millisecond
	c.BackoffMax = 50 * time.Millisecond
	c.RememberRoom("lobby")
	c.Start()
	defer c.Close()

	evs := collect(t, c, func(ev Event) bool {
		conn, ok := ev.(Connected)
		return ok && conn.Reconnected
	}, 5*time.Second)

	var sawReconnecting, sawReconnected bool
	for _, ev := range evs {
		switch e := ev.(type) {
		case Reconnecting:
			sawReconnecting = true
		case Connected:
			if e.Reconnected {
				sawReconnected = true
			}
		}
	}
	if !sawReconnecting || !sawReconnected {
		t.Fatalf("reconnecting=%v reconnected=%v", sawReconnecting, sawReconnected)
	}
	if !waitFor(t, 3*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		for _, j := range joined {
			if j == "|/join lobby" {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("client did not rejoin lobby: %#v", joined)
	}
	if srv.connectionCount() < 2 {
		t.Fatalf("expected at least 2 connections, got %d", srv.connectionCount())
	}
}

func TestChooseIncludesRqid(t *testing.T) {
	connCh := make(chan *websocket.Conn, 1)
	srv := newWSServer(t, func(conn *websocket.Conn) { connCh <- conn })

	c := NewClient(srv.url())
	c.Start()
	defer c.Close()

	// Wait until the client considers itself connected before sending.
	collect(t, c, func(ev Event) bool {
		_, ok := ev.(Connected)
		return ok
	}, 5*time.Second)

	var conn *websocket.Conn
	select {
	case conn = <-connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("no connection")
	}

	if err := c.Choose("battle-gen9randombattle-1", "move 1 +1 terastalize", 42); err != nil {
		t.Fatalf("Choose: %v", err)
	}
	if got := readCommand(t, conn, 5*time.Second); got != "battle-gen9randombattle-1|/choose move 1 +1 terastalize|42" {
		t.Fatalf("choose command = %q", got)
	}

	if err := c.Challenge("bob", "gen9randombattle"); err != nil {
		t.Fatalf("Challenge: %v", err)
	}
	if got := readCommand(t, conn, 5*time.Second); got != "|/challenge bob, gen9randombattle" {
		t.Fatalf("challenge command = %q", got)
	}
}

// TestOutboundMessagesAlwaysContainPipe guards the single easiest way to break
// every command at once: the live server drops messages without a room
// separator, so global commands must still be prefixed with "|".
func TestOutboundMessagesAlwaysContainPipe(t *testing.T) {
	connCh := make(chan *websocket.Conn, 1)
	srv := newWSServer(t, func(conn *websocket.Conn) { connCh <- conn })

	c := NewClient(srv.url())
	c.Start()
	defer c.Close()

	collect(t, c, func(ev Event) bool {
		_, ok := ev.(Connected)
		return ok
	}, 5*time.Second)

	var conn *websocket.Conn
	select {
	case conn = <-connCh:
	case <-time.After(5 * time.Second):
		t.Fatal("no connection")
	}

	cases := []struct {
		name string
		send func() error
		want string
	}{
		{"search", func() error { return c.Search("gen9randombattle") }, "|/search gen9randombattle"},
		{"cancel", func() error { return c.CancelSearch() }, "|/cancelsearch"},
		{"join", func() error { return c.JoinRoom("lobby") }, "|/join lobby"},
		{"whoami", func() error { return c.WhoAmI() }, "|/whoami"},
		{"team", func() error { return c.SendTeam("") }, "|/utm null"},
		{"challenge", func() error { return c.Challenge("bob", "gen9ou") }, "|/challenge bob, gen9ou"},
		{"accept", func() error { return c.AcceptChallenge("bob") }, "|/accept bob"},
		{"reject", func() error { return c.RejectChallenge("bob") }, "|/reject bob"},
		{"query", func() error { return c.Query("roomlist") }, "|/query roomlist"},
		{"choose", func() error { return c.Choose("battle-1", "move 1", 7) }, "battle-1|/choose move 1|7"},
		{"chat", func() error { return c.Chat("battle-1", "hi") }, "battle-1|hi"},
	}
	for _, tc := range cases {
		if err := tc.send(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got := readCommand(t, conn, 5*time.Second); got != tc.want {
			t.Errorf("%s: sent %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestBackoffIsBounded(t *testing.T) {
	min := 100 * time.Millisecond
	max := time.Second
	if got := backoff(1, min, max); got != min {
		t.Errorf("attempt 1 = %v, want %v", got, min)
	}
	if got := backoff(2, min, max); got != 2*min {
		t.Errorf("attempt 2 = %v, want %v", got, 2*min)
	}
	if got := backoff(50, min, max); got != max {
		t.Errorf("attempt 50 = %v, want %v", got, max)
	}
}
