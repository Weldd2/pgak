package config

import (
	"errors"

	"github.com/zalando/go-keyring"
)

// Passwords live in the OS keychain (macOS Keychain, Secret Service on linux,
// Credential Manager on windows), one entry per connection name. They are
// never written to the config file.
const keyringService = "pgak"

// StoredPassword returns the keychain password of a connection ("" if none).
func StoredPassword(name string) string {
	if name == "" {
		return ""
	}
	pw, err := keyring.Get(keyringService, name)
	if err != nil {
		return ""
	}
	return pw
}

// SetStoredPassword saves the password in the keychain; an empty password
// removes the entry.
func SetStoredPassword(name, password string) error {
	if password == "" {
		return DeleteStoredPassword(name)
	}
	return keyring.Set(keyringService, name, password)
}

// DeleteStoredPassword removes the keychain entry (no error if absent).
func DeleteStoredPassword(name string) error {
	if name == "" {
		return nil
	}
	err := keyring.Delete(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// UseMemoryKeyring swaps the OS keychain for an in-memory one (tests).
func UseMemoryKeyring() { keyring.MockInit() }
