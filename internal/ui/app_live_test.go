package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/anmura/pgak/internal/config"
	"github.com/anmura/pgak/internal/history"
)

// End-to-end against local postgres: PGAK_LIVE=1 go test ./internal/ui -run TestLiveFlow
func TestLiveFlow(t *testing.T) {
	if os.Getenv("PGAK_LIVE") != "1" {
		t.Skip("PGAK_LIVE != 1")
	}
	cfg := &config.Config{Connections: []config.Connection{
		{Name: "local", Host: "localhost", Port: 5432, User: os.Getenv("USER"), Database: "postgres"},
	}}
	a := NewApp(cfg, &history.History{})
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	// connect (run the command synchronously)
	msg := connectCmd(cfg.Connections[0], "")()
	a.Update(msg)
	if a.client == nil {
		t.Fatalf("not connected: %s", a.status)
	}
	if a.connName != "local" || a.database != "postgres" {
		t.Fatalf("conn=%s db=%s", a.connName, a.database)
	}

	// focus the editor, type a query and execute
	press(a, "enter")
	press(a, "i", "s", "e", "l", "e", "c", "t", " ", "4", "2", " ", "a", "s", " ", "n", "esc")
	_, cmd := a.runQuery()
	if cmd == nil {
		t.Fatal("expected query cmd")
	}
	a.Update(cmd())
	if a.statusErr {
		t.Fatalf("query failed: %s", a.status)
	}
	if len(a.res.Rows) != 1 || a.res.Rows[0][0] != "42" {
		t.Fatalf("rows: %v", a.res.Rows)
	}
	if a.zone != ZoneResults || !a.focused {
		t.Fatal("focus should move to results after query")
	}
	if len(a.hist.Entries) != 1 {
		t.Fatalf("history: %v", a.hist.Entries)
	}
	v := a.View()
	if !strings.Contains(v, "42") {
		t.Fatal("result not rendered")
	}

	// quick action: list tables
	_, cmd = a.handleQuickAction("sql:tables")
	if cmd == nil {
		t.Fatal("expected cmd")
	}
	a.Update(cmd())
	if a.statusErr {
		t.Fatalf("quick action failed: %s", a.status)
	}

	// the selected schema must be applied as search_path
	a.ed.SetValue("show search_path")
	_, cmd = a.runQuery()
	a.Update(cmd())
	if a.statusErr {
		t.Fatalf("show search_path failed: %s", a.status)
	}
	if len(a.res.Rows) != 1 || !strings.Contains(a.res.Rows[0][0], a.schema) {
		t.Fatalf("search_path=%v, schéma attendu %q", a.res.Rows, a.schema)
	}

	// the banner must reflect the schema
	if !strings.Contains(a.ed.View(), "search_path") {
		t.Fatal("banner not rendered in editor view")
	}
}
