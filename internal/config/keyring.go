package config

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// keyringService is the service name used for OS keychain entries.
const keyringService = "pokemon-slowdown"

// ErrNoStoredPassword means nothing was saved for this user.
var ErrNoStoredPassword = errors.New("config: no stored password")

// StorePassword saves a password in the OS keyring. It is only called when the
// user has explicitly opted in with remember = true.
func StorePassword(username, password string) error {
	return keyring.Set(keyringService, username, password)
}

// LoadPassword retrieves a stored password, or ErrNoStoredPassword.
func LoadPassword(username string) (string, error) {
	pw, err := keyring.Get(keyringService, username)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNoStoredPassword
	}
	return pw, err
}

// DeletePassword removes a stored password.
func DeletePassword(username string) error {
	err := keyring.Delete(keyringService, username)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
