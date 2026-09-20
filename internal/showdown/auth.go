package showdown

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LoginURL is the HTTP endpoint that exchanges credentials for an assertion.
const LoginURL = "https://play.pokemonshowdown.com/api/login"

// maxLoginResponse caps the login response body.
const maxLoginResponse = 1 << 20

// loginResponse is the shape of a successful /api/login response. The body is
// prefixed with ']' to defeat JSON hijacking and must be stripped first.
type loginResponse struct {
	Assertion string `json:"assertion"`
	CurUser   struct {
		LoggedIn bool   `json:"loggedin"`
		UserID   string `json:"userid"`
		Username string `json:"username"`
	} `json:"curuser"`
	ActionSuccess bool `json:"actionsuccess"`
}

// AuthFailed is emitted when a login attempt fails.
type AuthFailed struct {
	Base
	Err error
}

// Challstr returns the most recent login challenge, or "".
func (c *Client) Challstr() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.challstr
}

func (c *Client) authenticate(challstr string) {
	if err := c.Authenticate(challstr); err != nil {
		c.emit(AuthFailed{Err: err})
	}
}

// Authenticate performs the documented login flow for the stored credentials
// against the given challstr. An empty password performs a guest rename.
func (c *Client) Authenticate(challstr string) error {
	creds := c.Credentials()
	if creds == nil || creds.Username == "" {
		return nil
	}
	if creds.Password == "" {
		// Guest rename: no assertion required.
		return c.Send("/trn " + creds.Username)
	}

	assertion, err := FetchAssertion(challstr, creds.Username, creds.Password)
	if err != nil {
		return err
	}
	if assertion == "" {
		return fmt.Errorf("showdown: login rejected for %q", creds.Username)
	}
	return c.Send(fmt.Sprintf("/trn %s,0,%s", creds.Username, assertion))
}

// FetchAssertion exchanges a username, password and challstr for a login
// assertion. It never logs the password or the assertion.
func FetchAssertion(challstr, username, password string) (string, error) {
	return fetchAssertion(context.Background(), LoginURL, challstr, username, password, nil)
}

func fetchAssertion(ctx context.Context, endpoint, challstr, username, password string, hc *http.Client) (string, error) {
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	form := url.Values{}
	form.Set("name", username)
	form.Set("pass", password)
	form.Set("challstr", challstr)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLoginResponse))
	if err != nil {
		return "", err
	}
	if len(body) > 0 && body[0] == ']' {
		body = body[1:]
	}
	var lr loginResponse
	if err := json.Unmarshal(body, &lr); err != nil {
		return "", fmt.Errorf("showdown: unreadable login response: %w", err)
	}
	return lr.Assertion, nil
}

// Logout clears stored credentials and asks the server to drop our name.
func (c *Client) Logout() error {
	c.mu.Lock()
	c.creds = nil
	c.mu.Unlock()
	return c.Send("/trn")
}
