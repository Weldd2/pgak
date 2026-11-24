package config

import (
	"strings"
	"testing"
)

func TestPasswordKeychainThenEnv(t *testing.T) {
	UseMemoryKeyring()
	t.Setenv("PGAK_TEST_PW", "fromenv")
	c := Connection{Name: "c1", Host: "h", User: "u", PasswordEnv: "PGAK_TEST_PW"}

	if got := c.Password(); got != "fromenv" {
		t.Fatalf("env fallback: got %q", got)
	}
	if err := SetStoredPassword("c1", "s3cr et"); err != nil {
		t.Fatal(err)
	}
	if got := c.Password(); got != "s3cr et" {
		t.Fatalf("keychain should win: got %q", got)
	}
	if !strings.Contains(c.DSN(""), `password='s3cr et'`) {
		t.Fatalf("DSN: %s", c.DSN(""))
	}
	// empty password removes the entry; deleting twice is not an error
	if err := SetStoredPassword("c1", ""); err != nil {
		t.Fatal(err)
	}
	if err := DeleteStoredPassword("c1"); err != nil {
		t.Fatal(err)
	}
	if got := c.Password(); got != "fromenv" {
		t.Fatalf("after delete: got %q", got)
	}
}
