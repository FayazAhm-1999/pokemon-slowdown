package showdown

import "fmt"

// Commands that Pokémon Showdown understands. They are thin wrappers so the UI
// never has to build protocol strings by hand, and so usernames and battle IDs
// are never shell-interpolated anywhere.

// JoinRoom joins a chat or battle room and remembers it for reconnects.
func (c *Client) JoinRoom(roomID string) error {
	c.RememberRoom(roomID)
	return c.Send("/join " + roomID)
}

// LeaveRoom leaves a room and forgets it.
func (c *Client) LeaveRoom(roomID string) error {
	c.mu.Lock()
	delete(c.rooms, roomID)
	c.mu.Unlock()
	return c.Send("/leave " + roomID)
}

// RememberRoom records a room so it is rejoined after a reconnect.
func (c *Client) RememberRoom(roomID string) {
	if roomID == "" {
		return
	}
	c.mu.Lock()
	c.rooms[roomID] = true
	c.mu.Unlock()
}

// ForgetRoom stops rejoining a room.
func (c *Client) ForgetRoom(roomID string) {
	c.mu.Lock()
	delete(c.rooms, roomID)
	c.mu.Unlock()
}

// Search starts a ladder search for a format.
func (c *Client) Search(format string) error {
	return c.Send("/search " + format)
}

// CancelSearch cancels the current ladder search.
func (c *Client) CancelSearch() error {
	return c.Send("/cancelsearch")
}

// Challenge challenges a user to a format.
func (c *Client) Challenge(user, format string) error {
	return c.Send(fmt.Sprintf("/challenge %s, %s", user, format))
}

// AcceptChallenge accepts a challenge from a user.
func (c *Client) AcceptChallenge(user string) error {
	return c.Send("/accept " + user)
}

// RejectChallenge rejects a challenge from a user.
func (c *Client) RejectChallenge(user string) error {
	return c.Send("/reject " + user)
}

// CancelChallenge cancels an outgoing challenge.
func (c *Client) CancelChallenge(user string) error {
	return c.Send("/cancelchallenge " + user)
}

// SendTeam uploads a packed team for the next search or challenge.
func (c *Client) SendTeam(packed string) error {
	if packed == "" {
		packed = "null"
	}
	return c.Send("/utm " + packed)
}

// Choose submits a battle decision. rqid may be 0 when the server did not send
// one, in which case no request id is appended.
func (c *Client) Choose(roomID, choice string, rqid int) error {
	cmd := "/choose " + choice
	if rqid > 0 {
		cmd += fmt.Sprintf("|%d", rqid)
	}
	return c.SendTo(roomID, cmd)
}

// Chat sends a chat message to a room.
func (c *Client) Chat(roomID, message string) error {
	return c.SendTo(roomID, message)
}

// Query asks the server for structured data.
func (c *Client) Query(kind string, details ...string) error {
	cmd := "/query " + kind
	for _, d := range details {
		cmd += " " + d
	}
	return c.Send(cmd)
}

// WhoAmI asks the server to re-send our user details.
func (c *Client) WhoAmI() error { return c.Send("/whoami") }
