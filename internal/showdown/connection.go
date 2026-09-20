package showdown

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// DefaultURL is the public Pokémon Showdown websocket endpoint.
const DefaultURL = "wss://sim3.psim.us/showdown/websocket"

// maxMessageBytes caps an inbound websocket message. Server messages are
// treated as untrusted input.
const maxMessageBytes = 1 << 20

// Connected is emitted when the websocket is established.
type Connected struct {
	Base
	Reconnected bool
}

// Disconnected is emitted when the websocket drops.
type Disconnected struct {
	Base
	Err error
}

// Reconnecting is emitted while the client waits to retry.
type Reconnecting struct {
	Base
	Attempt int
	Delay   time.Duration
}

// Credentials are the login details a Client should use. An empty Password
// means a guest rename.
type Credentials struct {
	Username string
	Password string
}

// Client is a reconnecting Pokémon Showdown websocket client. It owns the
// socket, parses inbound frames into typed events and serialises outbound
// commands. It never logs credentials.
type Client struct {
	url    string
	events chan Event

	mu     sync.Mutex
	conn   *websocket.Conn
	creds  *Credentials
	rooms  map[string]bool
	closed bool

	// challstr is the most recent login challenge from the server.
	challstr string

	// connections counts successful dials, so a reconnect can be distinguished
	// from a first connection.
	connections int

	// BackoffMin and BackoffMax bound reconnect delays.
	BackoffMin time.Duration
	BackoffMax time.Duration

	// Debug enables verbose logging by the caller (the client itself never
	// logs secrets).
	Debug bool

	done      chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup
}

// NewClient returns a client for the given websocket URL. Pass DefaultURL for
// the public server.
func NewClient(url string) *Client {
	if url == "" {
		url = DefaultURL
	}
	return &Client{
		url:        url,
		events:     make(chan Event, 1024),
		rooms:      map[string]bool{},
		BackoffMin: time.Second,
		BackoffMax: 30 * time.Second,
		done:       make(chan struct{}),
	}
}

// Events returns the channel of typed server events. It is closed when the
// client is closed.
func (c *Client) Events() <-chan Event { return c.events }

// SetCredentials stores login details to use on the next |challstr|.
func (c *Client) SetCredentials(creds Credentials) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.creds = &creds
}

// Credentials returns the stored credentials, or nil.
func (c *Client) Credentials() *Credentials {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.creds
}

// Start launches the connection loop. It returns immediately.
func (c *Client) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.run()
	}()
}

// Close shuts the client down and closes the event channel.
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		conn := c.conn
		c.mu.Unlock()
		close(c.done)
		if conn != nil {
			// CloseNow is deliberate: a graceful close handshake can block for
			// seconds if the server is unresponsive, and we are exiting anyway.
			_ = conn.CloseNow()
		}
		c.wg.Wait()
		close(c.events)
	})
}

func (c *Client) isClosed() bool {
	select {
	case <-c.done:
		return true
	default:
		return false
	}
}

func (c *Client) run() {
	attempt := 0
	for {
		if c.isClosed() {
			return
		}
		err := c.connectAndRead()
		if c.isClosed() {
			return
		}
		attempt++
		delay := backoff(attempt, c.BackoffMin, c.BackoffMax)
		c.emit(Reconnecting{Attempt: attempt, Delay: delay})
		select {
		case <-time.After(delay):
		case <-c.done:
			return
		}
		_ = err
	}
}

// connectAndRead dials, reads until error, and reports the error.
func (c *Client) connectAndRead() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	conn, _, err := websocket.Dial(ctx, c.url, &websocket.DialOptions{
		HTTPHeader: map[string][]string{"User-Agent": {"pokemon-slowdown"}},
	})
	cancel()
	if err != nil {
		return err
	}
	conn.SetReadLimit(maxMessageBytes)

	c.mu.Lock()
	reconnected := c.connections > 0
	c.connections++
	c.conn = conn
	c.mu.Unlock()

	c.emit(Connected{Reconnected: reconnected})
	c.rejoin()

	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
		_ = conn.CloseNow()
	}()

	for {
		_, data, err := conn.Read(context.Background())
		if err != nil {
			if !c.isClosed() {
				c.emit(Disconnected{Err: err})
			}
			return err
		}
		for _, frame := range SplitFrames(string(data)) {
			for _, ev := range Parse(frame) {
				if cs, ok := ev.(ChallStr); ok {
					c.mu.Lock()
					c.challstr = cs.Challstr
					c.mu.Unlock()
					go c.authenticate(cs.Challstr)
				}
				c.emit(ev)
			}
		}
	}
}

// emit delivers an event unless the client is shutting down.
func (c *Client) emit(ev Event) {
	select {
	case c.events <- ev:
	case <-c.done:
	}
}

// rejoin re-subscribes to the rooms we were in after a reconnect.
func (c *Client) rejoin() {
	c.mu.Lock()
	rooms := make([]string, 0, len(c.rooms))
	for r := range c.rooms {
		rooms = append(rooms, r)
	}
	c.mu.Unlock()
	for _, r := range rooms {
		_ = c.Send("/join " + r)
	}
}

// Send writes a raw command to the server. roomID may be empty for global
// commands.
func (c *Client) Send(command string) error { return c.SendTo("", command) }

// SendTo writes a command scoped to a room.
//
// The room separator is required even when the room is empty: the server drops
// any message without a pipe ("messages should be in the format
// ROOMID|MESSAGE"). A global command is therefore sent as "|/search ...", not
// "/search ...".
func (c *Client) SendTo(roomID, command string) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return errors.New("showdown: not connected")
	}
	msg := roomID + "|" + command
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, []byte(msg))
}

// backoff returns an exponential delay bounded by max.
func backoff(attempt int, min, max time.Duration) time.Duration {
	if min <= 0 {
		min = time.Second
	}
	if max <= 0 {
		max = 30 * time.Second
	}
	d := min
	for i := 1; i < attempt && d < max; i++ {
		d *= 2
	}
	if d > max {
		d = max
	}
	return d
}
