package ui

import (
	"os"
	"testing"

	"pgak/internal/config"
)

func TestMain(m *testing.M) {
	config.UseMemoryKeyring() // never touch the real OS keychain in tests
	os.Exit(m.Run())
}
