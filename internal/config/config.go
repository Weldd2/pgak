package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Connection describes a saved postgres connection.
// The password is never stored directly: PasswordEnv holds the name of an
// environment variable containing the password.
type Connection struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	User        string `json:"user"`
	PasswordEnv string `json:"password_env,omitempty"`
	Database    string `json:"database,omitempty"`
	SSLMode     string `json:"sslmode,omitempty"`
}

// Password resolves the password from the environment.
func (c Connection) Password() string {
	if c.PasswordEnv == "" {
		return ""
	}
	return os.Getenv(c.PasswordEnv)
}

// DSN builds a connection string for the given database (falls back to the
// connection's default database, then to "postgres"). Values are quoted so
// spaces and special characters are safe.
func (c Connection) DSN(database string) string {
	db := database
	if db == "" {
		db = c.Database
	}
	if db == "" {
		db = "postgres"
	}
	port := c.Port
	if port == 0 {
		port = 5432
	}
	ssl := c.SSLMode
	if ssl == "" {
		ssl = "prefer"
	}
	parts := []string{
		kv("host", c.Host),
		kv("port", fmt.Sprintf("%d", port)),
		kv("user", c.User),
		kv("dbname", db),
		kv("sslmode", ssl),
	}
	if pw := c.Password(); pw != "" {
		parts = append(parts, kv("password", pw))
	}
	return strings.Join(parts, " ")
}

// kv renders a key=value DSN pair, quoting the value when needed.
func kv(k, v string) string {
	if v == "" || strings.ContainsAny(v, " '\\") {
		v = "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
	}
	return k + "=" + v
}

type Config struct {
	Connections []Connection `json:"connections"`
}

func dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "pgak"), nil
}

// Path returns the config file path (~/.config/pgak/config.json on linux,
// ~/Library/Application Support/pgak/config.json on macOS unless XDG set).
func Path() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "config.json"), nil
}

// HistoryPath returns the query history file path.
func HistoryPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "history.json"), nil
}

// LogPath returns the log file path.
func LogPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "pgak.log"), nil
}

// Load reads the config file. A missing file yields an empty config.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config invalide (%s): %w", p, err)
	}
	return &cfg, nil
}

// Save writes the config file, creating the directory if needed.
func (cfg *Config) Save() error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o600)
}

// Upsert inserts or replaces a connection by name (oldName = previous name,
// empty for a new connection).
func (cfg *Config) Upsert(oldName string, c Connection) {
	if oldName != "" {
		for i := range cfg.Connections {
			if cfg.Connections[i].Name == oldName {
				cfg.Connections[i] = c
				return
			}
		}
	}
	cfg.Connections = append(cfg.Connections, c)
}

// Delete removes a connection by name.
func (cfg *Config) Delete(name string) {
	out := cfg.Connections[:0]
	for _, c := range cfg.Connections {
		if c.Name != name {
			out = append(out, c)
		}
	}
	cfg.Connections = out
}

// Get returns a connection by name.
func (cfg *Config) Get(name string) (Connection, bool) {
	for _, c := range cfg.Connections {
		if c.Name == name {
			return c, true
		}
	}
	return Connection{}, false
}
