package ui

import (
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pgak/internal/config"
	"pgak/internal/history"
	"pgak/internal/i18n"
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

	// several statements → one tab each, errors in their own tab
	a.ed.SetValue("select 1 as a;\nselect * from table_that_does_not_exist;\nselect 3 as c;")
	_, cmd = a.runQuery()
	a.Update(cmd())
	if len(a.tabs) != 3 {
		t.Fatalf("tabs=%d status=%s", len(a.tabs), a.status)
	}
	if a.curTab != 1 || !a.tabs[1].err || a.res.Err == "" {
		t.Fatalf("failing tab should be selected: cur=%d err=%q", a.curTab, a.res.Err)
	}
	if a.tabs[0].res.Rows[0][0] != "1" || a.tabs[2].res.Rows[0][0] != "3" {
		t.Fatalf("tab rows: %v / %v", a.tabs[0].res.Rows, a.tabs[2].res.Rows)
	}
	press(a, "L")
	if a.curTab != 2 || a.res.Rows[0][0] != "3" {
		t.Fatalf("L: cur=%d rows=%v", a.curTab, a.res.Rows)
	}
	if !strings.Contains(a.View(), "1:select 1 as a") {
		t.Fatal("tab bar not rendered")
	}

	// the banner must reflect the schema
	if !strings.Contains(a.ed.View(), "search_path") {
		t.Fatal("banner not rendered in editor view")
	}
}

// Cell editing from the UI against local postgres: PGAK_LIVE=1 go test ./internal/ui -run TestLiveCellEdit
func TestLiveCellEdit(t *testing.T) {
	if os.Getenv("PGAK_LIVE") != "1" {
		t.Skip("PGAK_LIVE != 1")
	}
	i18n.SetLanguage("fr")
	cfg := &config.Config{Language: "fr", Connections: []config.Connection{
		{Name: "local", Host: "localhost", Port: 5432, User: os.Getenv("USER"), Database: "postgres"},
	}}
	a := NewApp(cfg, &history.History{})
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	a.Update(connectCmd(cfg.Connections[0], "")())
	if a.client == nil {
		t.Fatalf("not connected: %s", a.status)
	}
	run := func(sql string) {
		a.ed.SetValue(sql)
		_, cmd := a.runQuery()
		a.Update(cmd())
		if a.statusErr {
			t.Fatalf("%s: %s", sql, a.status)
		}
	}
	run(`CREATE TEMP TABLE pgak_ui_edit (id int PRIMARY KEY, name text)`)
	run(`INSERT INTO pgak_ui_edit VALUES (1, 'one'), (2, 'two')`)
	run(`SELECT id, name, length(name) AS len FROM pgak_ui_edit ORDER BY id`)
	if a.zone != ZoneResults || !a.focused {
		t.Fatal("results should be focused")
	}
	// edit name of row 2: "two" → "twoX"
	press(a, "j", "l", "i", "X", "enter")
	if !a.busy {
		t.Fatalf("update should be running: %s", a.status)
	}
	// run the pending command synchronously
	_, cmd := a.applyCellEdit(&CellEdit{Row: 1, Col: 1, Value: "twoX"})
	a.Update(cmd())
	if a.statusErr || a.res.cell(1, 1) != "twoX" {
		t.Fatalf("status=%q cell=%q", a.status, a.res.cell(1, 1))
	}
	run(`SELECT name FROM pgak_ui_edit WHERE id = 2`)
	if a.res.cell(0, 0) != "twoX" {
		t.Fatalf("not persisted: %v", a.res.Rows)
	}
	// computed column: refused with a message
	run(`SELECT id, name, length(name) AS len FROM pgak_ui_edit ORDER BY id`)
	press(a, "$", "i")
	if a.res.Editing() || !strings.Contains(a.status, "Non éditable") {
		t.Fatalf("computed column: editing=%v status=%q", a.res.Editing(), a.status)
	}
}
