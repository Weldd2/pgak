package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Session is the state restored at startup: last connection, database,
// schema and the content of the query editor.
type Session struct {
	Connection string `json:"connection,omitempty"`
	Database   string `json:"database,omitempty"`
	Schema     string `json:"schema,omitempty"`
	Query      string `json:"query,omitempty"`
}

// SessionPath returns the session file path.
func SessionPath() (string, error) {
	d, err := dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(d, "session.json"), nil
}

// LoadSession reads the last saved session. A missing file yields nil.
func LoadSession() (*Session, error) {
	p, err := SessionPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, nil // corrupt session: start fresh
	}
	return &s, nil
}

// Save writes the session file, creating the directory if needed.
func (s *Session) Save() error {
	p, err := SessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o600)
}
