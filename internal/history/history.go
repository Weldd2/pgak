package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"pgak/internal/config"
)

const maxEntries = 500

// Entry is a previously executed query.
type Entry struct {
	Query      string    `json:"query"`
	Connection string    `json:"connection"`
	Database   string    `json:"database"`
	At         time.Time `json:"at"`
}

type History struct {
	Entries []Entry `json:"entries"` // most recent first
}

func Load() (*History, error) {
	p, err := config.HistoryPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &History{}, nil
		}
		return nil, err
	}
	var h History
	if err := json.Unmarshal(data, &h); err != nil {
		// corrupted history should never block the app
		return &History{}, nil
	}
	return &h, nil
}

func (h *History) Save() error {
	p, err := config.HistoryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o600)
}

// Add prepends a query, deduplicating identical consecutive entries.
func (h *History) Add(query, connection, database string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return
	}
	// drop previous identical entries
	out := make([]Entry, 0, len(h.Entries)+1)
	out = append(out, Entry{Query: q, Connection: connection, Database: database, At: time.Now()})
	for _, e := range h.Entries {
		if e.Query != q {
			out = append(out, e)
		}
	}
	if len(out) > maxEntries {
		out = out[:maxEntries]
	}
	h.Entries = out
}
