package ui

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pgak/internal/config"
	"pgak/internal/db"
	"pgak/internal/history"
	"pgak/internal/i18n"
)

func newTestApp() *App {
	i18n.SetLanguage("fr") // tests assert French labels
	cfg := &config.Config{
		Language: "fr", // skip the first-launch language picker
		Connections: []config.Connection{
			{Name: "local", Host: "localhost", Port: 5432, User: "me"},
		},
	}
	a := NewApp(cfg, &history.History{})
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return a
}

func press(a *App, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "ctrl+t":
			msg = tea.KeyMsg{Type: tea.KeyCtrlT}
		case "ctrl+p":
			msg = tea.KeyMsg{Type: tea.KeyCtrlP}
		case "ctrl+e":
			msg = tea.KeyMsg{Type: tea.KeyCtrlE}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		a.Update(msg)
	}
}

func TestViewRendersAllZones(t *testing.T) {
	a := newTestApp()
	v := a.View()
	if !strings.Contains(v, "Requête") || !strings.Contains(v, "Résultats") {
		t.Fatal("missing zones in view")
	}
}

func TestInitOpensConnModal(t *testing.T) {
	a := newTestApp()
	a.Init()
	if a.modal == nil || a.modalPurp != purpConn {
		t.Fatal("expected connection modal at init")
	}
	v := a.View()
	if !strings.Contains(v, "Connexions") {
		t.Fatal("modal not rendered")
	}
	// esc in insert -> normal mode, esc again -> close
	press(a, "esc", "esc")
	if a.modal != nil {
		t.Fatal("modal should be closed after esc esc")
	}
}

func TestWindowSelectionMode(t *testing.T) {
	a := newTestApp()
	if a.focused {
		t.Fatal("should start in window-selection mode")
	}
	if a.zone != ZoneEditor {
		t.Fatal("default zone should be editor")
	}
	// hjkl move between windows
	press(a, "j")
	if a.zone != ZoneResults {
		t.Fatalf("j: zone=%d", a.zone)
	}
	press(a, "k", "k")
	if a.zone != ZoneHeader {
		t.Fatalf("kk: zone=%d", a.zone)
	}
	press(a, "k") // clamped at top
	if a.zone != ZoneHeader {
		t.Fatalf("clamp: zone=%d", a.zone)
	}
	// enter focuses, esc returns to window selection
	press(a, "j", "enter")
	if !a.focused || a.zone != ZoneEditor {
		t.Fatalf("enter: focused=%v zone=%d", a.focused, a.zone)
	}
	press(a, "esc")
	if a.focused {
		t.Fatal("esc should return to window-selection mode")
	}
	a.View()
}

func TestEscFromEditorModes(t *testing.T) {
	a := newTestApp()
	press(a, "enter")  // focus editor
	press(a, "i", "x") // insert mode
	press(a, "esc")    // insert -> normal, still focused
	if !a.focused {
		t.Fatal("esc from insert must stay focused")
	}
	press(a, "v")   // visual
	press(a, "esc") // visual -> normal, still focused
	if !a.focused {
		t.Fatal("esc from visual must stay focused")
	}
	press(a, "esc") // normal -> window selection
	if a.focused {
		t.Fatal("esc from normal must exit to window selection")
	}
}

func TestZoneCycling(t *testing.T) {
	a := newTestApp()
	press(a, "tab")
	if a.zone != ZoneResults {
		t.Fatalf("zone=%d", a.zone)
	}
	press(a, "tab")
	if a.zone != ZoneHeader {
		t.Fatalf("zone=%d", a.zone)
	}
	a.View()
}

func TestQuickActionsModal(t *testing.T) {
	a := newTestApp()
	press(a, "ctrl+t")
	if a.modal == nil || a.modalPurp != purpQuick {
		t.Fatal("expected quick actions modal")
	}
	a.View()
}

func TestHistoryModal(t *testing.T) {
	a := newTestApp()
	a.hist.Add("select 1", "local", "postgres")
	press(a, "ctrl+p")
	if a.modal == nil || a.modalPurp != purpHistory {
		t.Fatal("expected history modal")
	}
	press(a, "enter")
	if a.modal != nil {
		t.Fatal("modal should close on enter")
	}
	if !strings.Contains(a.ed.Value(), "select 1") {
		t.Fatalf("editor value %q", a.ed.Value())
	}
}

func TestEditorTyping(t *testing.T) {
	a := newTestApp()
	press(a, "enter") // focus editor
	press(a, "i", "s", "e", "l")
	if a.ed.Value() != "sel" {
		t.Fatalf("got %q", a.ed.Value())
	}
	press(a, "esc")
	a.View()
}

func TestResultsGGScrollAndCounts(t *testing.T) {
	r := NewResults()
	r.SetSize(80, 8) // pageSize = 6
	rows := make([][]string, 50)
	for i := range rows {
		rows[i] = []string{strconv.Itoa(i)}
	}
	r.SetData([]string{"n"}, rows)
	k := func(keys ...string) {
		for _, s := range keys {
			r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		}
	}
	k("G")
	if r.curRow != 49 || r.scrollRow == 0 {
		t.Fatalf("G: curRow=%d scrollRow=%d", r.curRow, r.scrollRow)
	}
	k("g", "g")
	if r.curRow != 0 {
		t.Fatalf("gg: curRow=%d", r.curRow)
	}
	if r.scrollRow != 0 {
		t.Fatalf("gg must scroll back up: scrollRow=%d", r.scrollRow)
	}
	k("1", "0", "j")
	if r.curRow != 10 {
		t.Fatalf("10j: curRow=%d", r.curRow)
	}
	k("4", "2", "G")
	if r.curRow != 41 {
		t.Fatalf("42G: curRow=%d", r.curRow)
	}
	k("5", "g", "g")
	if r.curRow != 4 {
		t.Fatalf("5gg: curRow=%d", r.curRow)
	}
	r.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if r.curRow != 4+r.pageSize() {
		t.Fatalf("ctrl+f: curRow=%d", r.curRow)
	}
	r.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if r.curRow != 4 {
		t.Fatalf("ctrl+b: curRow=%d", r.curRow)
	}
}

func TestResultsSearch(t *testing.T) {
	r := NewResults()
	r.SetSize(80, 8)
	r.SetData([]string{"id", "name"}, [][]string{
		{"1", "alice"},
		{"2", "bob"},
		{"3", "alicia"},
		{"4", "carol"},
	})
	k := func(keys ...string) {
		for _, s := range keys {
			switch s {
			case "enter":
				r.Update(tea.KeyMsg{Type: tea.KeyEnter})
			case "esc":
				r.Update(tea.KeyMsg{Type: tea.KeyEsc})
			default:
				r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
			}
		}
	}
	// / opens the prompt, typing jumps incrementally (case-insensitive)
	k("/")
	if !r.searching || r.PendingKeys() == "" {
		t.Fatal("/ should enter search mode")
	}
	k("ALI", "enter")
	if r.searching {
		t.Fatal("enter should close the prompt")
	}
	if r.curRow != 0 || r.curCol != 1 {
		t.Fatalf("search ali: cur=(%d,%d)", r.curRow, r.curCol)
	}
	// n / N cycle through matches with wrap-around
	k("n")
	if r.curRow != 2 || r.curCol != 1 {
		t.Fatalf("n: cur=(%d,%d)", r.curRow, r.curCol)
	}
	k("n") // wraps back to the first match
	if r.curRow != 0 || r.curCol != 1 {
		t.Fatalf("n wrap: cur=(%d,%d)", r.curRow, r.curCol)
	}
	k("N")
	if r.curRow != 2 || r.curCol != 1 {
		t.Fatalf("N: cur=(%d,%d)", r.curRow, r.curCol)
	}
	// esc cancels and restores the original position
	k("g", "g", "0", "/", "bob", "esc")
	if r.searching || r.searchQuery != "" {
		t.Fatal("esc should cancel the search")
	}
	if r.curRow != 0 || r.curCol != 0 {
		t.Fatalf("esc should restore position: cur=(%d,%d)", r.curRow, r.curCol)
	}
	// no match: status message, cursor untouched
	k("/", "zzz", "enter")
	if r.Status == "" {
		t.Fatal("expected a no-match status")
	}
	if r.curRow != 0 || r.curCol != 0 {
		t.Fatalf("no match must not move: cur=(%d,%d)", r.curRow, r.curCol)
	}
}

func TestResultsNavigationAndRender(t *testing.T) {
	a := newTestApp()
	a.res.SetData([]string{"id", "name"}, [][]string{{"1", "alice"}, {"2", "bob"}})
	press(a, "j", "enter") // select results window and focus it
	if a.zone != ZoneResults || !a.focused {
		t.Fatalf("zone=%d focused=%v", a.zone, a.focused)
	}
	press(a, "j", "l")
	v := a.View()
	if !strings.Contains(v, "alice") {
		t.Fatal("results not rendered")
	}
}

func TestResultsHelpOverlay(t *testing.T) {
	a := newTestApp()
	a.res.SetData([]string{"id"}, [][]string{{"1"}})
	// from window-selection mode: ? opens the window-mode help
	press(a, "j", "?")
	if a.showHelp != helpWindow {
		t.Fatalf("? should open the window help from window-selection mode, got %q", a.showHelp)
	}
	v := a.View()
	for _, s := range []string{"ctrl+o", "ctrl+t", "Fenêtres", "Navigation"} {
		if !strings.Contains(v, s) {
			t.Fatalf("window help missing %q", s)
		}
	}
	press(a, "esc") // any key closes
	if a.showHelp != helpNone {
		t.Fatal("help should close on any key")
	}
	// from the focused results window
	press(a, "enter", "?")
	if a.showHelp != helpResults {
		t.Fatalf("? should open the results help in focused results, got %q", a.showHelp)
	}
	v = a.View()
	for _, s := range []string{"yc", "yY", "ya", "Copier", "Navigation"} {
		if !strings.Contains(v, s) {
			t.Fatalf("results help missing %q", s)
		}
	}
	press(a, "j")
	if a.showHelp != helpNone {
		t.Fatal("help should close on any key")
	}
	if a.res.PendingKeys() != "" {
		t.Fatal("closing key must not leak into results")
	}
	// ? must not open help from the editor (it is a typed character)
	press(a, "esc", "k", "enter", "i", "?")
	if a.showHelp != helpNone {
		t.Fatal("? in editor insert mode must not open help")
	}
	if !strings.Contains(a.ed.Value(), "?") {
		t.Fatalf("editor should contain '?', got %q", a.ed.Value())
	}
}

func TestOpenConfigEditorShortcut(t *testing.T) {
	a := newTestApp()
	t.Setenv("EDITOR", "true") // /usr/bin/true: exits immediately
	msg := tea.KeyMsg{Type: tea.KeyCtrlO}
	_, cmd := a.Update(msg)
	if cmd == nil {
		t.Fatal("ctrl+o should return an ExecProcess command")
	}
}

func TestInitialConnectionUnknown(t *testing.T) {
	a := newTestApp()
	a.SetInitialConnection("nope")
	cmd := a.Init()
	if cmd != nil {
		t.Fatal("unknown connection must not produce a connect cmd")
	}
	if !a.statusErr || !strings.Contains(a.status, "nope") {
		t.Fatalf("expected error status, got %q", a.status)
	}
	if a.modal == nil || a.modalPurp != purpConn {
		t.Fatal("should fall back to the connection picker")
	}
}

func TestInitialConnectionKnown(t *testing.T) {
	a := newTestApp()
	a.SetInitialConnection("local")
	cmd := a.Init()
	if cmd == nil {
		t.Fatal("expected a connect cmd for a known connection")
	}
	if a.modal != nil {
		t.Fatal("picker should not open when --connexion is used")
	}
	if !a.busy {
		t.Fatal("app should be busy while connecting")
	}
}

func TestRestoreSession(t *testing.T) {
	a := newTestApp()
	a.RestoreSession(&config.Session{Connection: "local", Database: "appdb", Schema: "ventes", Query: "select 1"})
	if a.ed.Value() != "select 1" {
		t.Fatalf("query not restored: %q", a.ed.Value())
	}
	if a.schema != "ventes" {
		t.Fatalf("schema not restored: %q", a.schema)
	}
	cmd := a.Init()
	if cmd == nil {
		t.Fatal("expected a connect cmd from the restored session")
	}
	if a.initialDB != "appdb" {
		t.Fatalf("database not restored: %q", a.initialDB)
	}
	if a.modal != nil {
		t.Fatal("picker should not open when a session is restored")
	}
}

func TestRestoreSessionUnknownConnection(t *testing.T) {
	a := newTestApp()
	a.RestoreSession(&config.Session{Connection: "disparue", Query: "select 2"})
	if a.initialConn != "" {
		t.Fatal("unknown connection must not be restored")
	}
	if cmd := a.Init(); cmd != nil {
		t.Fatal("no connect cmd expected")
	}
	if a.modal == nil || a.modalPurp != purpConn {
		t.Fatal("should fall back to the connection picker")
	}
	if a.ed.Value() != "select 2" {
		t.Fatal("query should still be restored")
	}
}

func TestConnexionFlagBeatsSession(t *testing.T) {
	a := newTestApp()
	a.SetInitialConnection("local")
	a.RestoreSession(&config.Session{Connection: "autre", Database: "x"})
	if a.initialConn != "local" || a.initialDB != "" {
		t.Fatalf("--connexion must win: conn=%q db=%q", a.initialConn, a.initialDB)
	}
}

func TestQuitSavesSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // isolate os.UserConfigDir
	a := newTestApp()
	a.connName, a.database, a.schema = "local", "appdb", "ventes"
	press(a, "enter", "i", "s", "esc") // type "s" in the editor
	press(a, "esc", "q", "enter")      // back to window mode, quit (confirmed)
	s, err := config.LoadSession()
	if err != nil || s == nil {
		t.Fatalf("session not saved: %v", err)
	}
	if s.Connection != "local" || s.Database != "appdb" || s.Schema != "ventes" || s.Query != "s" {
		t.Fatalf("session=%+v", s)
	}
}

func TestQuitConfirmation(t *testing.T) {
	a := newTestApp()
	// q in window mode opens the confirmation, esc cancels
	press(a, "q")
	if !a.confirmQuit {
		t.Fatal("q should open the quit confirmation")
	}
	if !strings.Contains(a.View(), "Quitter pgak ?") {
		t.Fatal("confirmation overlay not rendered")
	}
	press(a, "esc")
	if a.confirmQuit {
		t.Fatal("esc should cancel the quit confirmation")
	}
	// n cancels too
	press(a, "q", "n")
	if a.confirmQuit {
		t.Fatal("n should cancel the quit confirmation")
	}
	// y confirms
	press(a, "q")
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("y should quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected a tea.Quit command")
	}
}

func TestQQuitsWithoutConfirmation(t *testing.T) {
	a := newTestApp()
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Q")})
	if cmd == nil {
		t.Fatal("Q should quit immediately")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("expected a tea.Quit command")
	}
	if a.confirmQuit {
		t.Fatal("Q must not open the confirmation")
	}
}

func TestQuitConfirmationFromResults(t *testing.T) {
	a := newTestApp()
	a.res.SetData([]string{"id"}, [][]string{{"1"}})
	press(a, "j", "enter") // focus results
	press(a, "q")
	if !a.confirmQuit {
		t.Fatal("q in results should open the quit confirmation")
	}
	press(a, "n")
	if a.confirmQuit {
		t.Fatal("n should cancel")
	}
}

func TestConnFormValidation(t *testing.T) {
	f := NewConnForm(nil)
	f.SetSize(60, 20)
	// ctrl+s with empty name should show an error, not close
	res := f.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if res != nil {
		t.Fatal("form should not close with invalid data")
	}
	if f.Err == "" {
		t.Fatal("expected a validation error")
	}
	f.View()
}

func TestSmallTerminalNoPanic(t *testing.T) {
	a := newTestApp()
	a.Update(tea.WindowSizeMsg{Width: 20, Height: 8})
	a.View()
	press(a, "tab", "tab", "tab")
	a.View()
}

// multiRun feeds the app a finished multi-statement run (no database needed).
func multiRun(a *App, results ...stmtResult) {
	sqls := make([]string, len(results))
	for i, r := range results {
		sqls[i] = r.sql
	}
	a.Update(queryDoneMsg{query: strings.Join(sqls, ";\n"), results: results})
}

func okStmt(sql string, cols []string, rows ...[]string) stmtResult {
	return stmtResult{sql: sql, res: &db.Result{Columns: cols, Rows: rows, IsSelect: true}}
}

func TestResultTabsFromMultipleStatements(t *testing.T) {
	a := newTestApp()
	a.connName, a.database = "local", "appdb"
	multiRun(a,
		okStmt("select 1 as one", []string{"one"}, []string{"1"}),
		okStmt("select 'deux' as two", []string{"two"}, []string{"deux"}),
		okStmt("select 3 as three", []string{"three"}, []string{"3"}),
	)
	if len(a.tabs) != 3 || a.curTab != 0 {
		t.Fatalf("tabs=%d cur=%d", len(a.tabs), a.curTab)
	}
	if a.zone != ZoneResults || !a.focused {
		t.Fatal("focus should move to results")
	}
	if a.statusErr || !strings.Contains(a.status, "3 requêtes") {
		t.Fatalf("status=%q err=%v", a.status, a.statusErr)
	}
	if len(a.hist.Entries) != 1 || !strings.Contains(a.hist.Entries[0].Query, "select 3") {
		t.Fatalf("history should hold the whole editor text: %+v", a.hist.Entries)
	}
	v := a.View()
	for _, s := range []string{"1:select 1 as one", "2:select 'deux' as …", "3:select 3 as three"} {
		if !strings.Contains(v, s) {
			t.Fatalf("tab bar missing %q in:\n%s", s, v)
		}
	}
	if !strings.Contains(v, "one") || strings.Contains(v, "deux") && !strings.Contains(v, "2:select") {
		t.Fatal("first tab should be displayed")
	}

	// L / H move right / left with wrap-around
	press(a, "L")
	if a.curTab != 1 || a.res != a.tabs[1].res {
		t.Fatalf("L: cur=%d", a.curTab)
	}
	if !strings.Contains(a.View(), "deux") {
		t.Fatal("second tab content not rendered")
	}
	press(a, "L", "L")
	if a.curTab != 0 {
		t.Fatalf("L wrap: cur=%d", a.curTab)
	}
	press(a, "H")
	if a.curTab != 2 {
		t.Fatalf("H wrap: cur=%d", a.curTab)
	}
	press(a, "H")
	if a.curTab != 1 {
		t.Fatalf("H: cur=%d", a.curTab)
	}
	// the tab bar takes one line off the table
	if a.res.height != a.tabs[1].res.height || a.res.height <= 0 {
		t.Fatalf("height=%d", a.res.height)
	}
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	_, _, resH := a.zoneHeights()
	if a.res.height != resH-3 {
		t.Fatalf("results height %d, want %d (tab bar line)", a.res.height, resH-3)
	}
}

func TestSingleStatementNoTabBar(t *testing.T) {
	a := newTestApp()
	multiRun(a, okStmt("select 1", []string{"n"}, []string{"1"}))
	if len(a.tabs) != 1 || a.showTabBar() {
		t.Fatalf("tabs=%d bar=%v", len(a.tabs), a.showTabBar())
	}
	if strings.Contains(a.View(), "1:select 1") {
		t.Fatal("tab bar must not render for a single result")
	}
	_, _, resH := a.zoneHeights()
	if a.res.height != resH-2 {
		t.Fatalf("results height %d, want %d", a.res.height, resH-2)
	}
	press(a, "L", "H") // no-ops
	if a.curTab != 0 {
		t.Fatal("H/L should be no-ops with one tab")
	}
}

func TestResultTabsWithErrors(t *testing.T) {
	a := newTestApp()
	multiRun(a,
		okStmt("select 1", []string{"n"}, []string{"1"}),
		stmtResult{sql: "select * from nope", err: errors.New(`relation "nope" does not exist`)},
		okStmt("select 3", []string{"n"}, []string{"3"}),
	)
	if len(a.tabs) != 3 {
		t.Fatalf("tabs=%d", len(a.tabs))
	}
	if a.curTab != 1 {
		t.Fatalf("the first failing tab should be selected, got %d", a.curTab)
	}
	if !a.statusErr || !strings.Contains(a.status, "1/3") {
		t.Fatalf("status=%q", a.status)
	}
	v := a.View()
	if !strings.Contains(v, `relation "nope" does not exist`) {
		t.Fatalf("error tab should show the message:\n%s", v)
	}
	if !strings.Contains(a.resInfo, "erreur") {
		t.Fatalf("resInfo=%q", a.resInfo)
	}
	// navigation keys on an error tab must not panic
	press(a, "j", "k", "g", "g", "G", "y", "y")
	press(a, "L")
	if a.curTab != 2 || !strings.Contains(a.View(), "3") {
		t.Fatalf("L: cur=%d", a.curTab)
	}
	// history is still recorded since at least one statement succeeded
	if len(a.hist.Entries) != 1 {
		t.Fatalf("history=%d", len(a.hist.Entries))
	}
}

func TestSingleFailingStatementKeepsPreviousResults(t *testing.T) {
	a := newTestApp()
	multiRun(a, okStmt("select 1", []string{"n"}, []string{"1"}))
	press(a, "esc", "k", "enter") // back to the editor
	multiRun(a, stmtResult{sql: "select nope", err: errors.New("boom")})
	if !a.statusErr || a.status != "boom" {
		t.Fatalf("status=%q", a.status)
	}
	if len(a.tabs) != 1 || a.res.Err != "" || len(a.res.Rows) != 1 {
		t.Fatal("previous results must be kept on a single failing statement")
	}
	if a.zone != ZoneEditor {
		t.Fatal("focus must stay in the editor")
	}
}

func TestResultsScreenKeysJK(t *testing.T) {
	r := NewResults()
	r.SetSize(80, 8) // pageSize = 6
	rows := make([][]string, 50)
	for i := range rows {
		rows[i] = []string{strconv.Itoa(i)}
	}
	r.SetData([]string{"n"}, rows)
	k := func(keys ...string) {
		for _, s := range keys {
			r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
		}
	}
	k("2", "0", "j") // row 20, scrolled so that 20 is the last visible row
	if r.curRow != 20 || r.scrollRow != 15 {
		t.Fatalf("20j: cur=%d scroll=%d", r.curRow, r.scrollRow)
	}
	k("K")
	if r.curRow != 15 {
		t.Fatalf("K (top of screen): cur=%d", r.curRow)
	}
	k("J")
	if r.curRow != 20 {
		t.Fatalf("J (bottom of screen): cur=%d", r.curRow)
	}
	k("M")
	if r.curRow != 18 {
		t.Fatalf("M: cur=%d", r.curRow)
	}
	// H / L no longer move inside the table
	k("H", "L")
	if r.curRow != 18 {
		t.Fatalf("H/L must not move the cursor: cur=%d", r.curRow)
	}
}

func TestResultsCellEdit(t *testing.T) {
	r := NewResults()
	r.SetSize(80, 8)
	r.SetData([]string{"id", "name"}, [][]string{{"1", "ab"}, {"2", "cd"}})
	k := func(keys ...string) {
		for _, s := range keys {
			switch s {
			case "enter":
				r.Update(tea.KeyMsg{Type: tea.KeyEnter})
			case "esc":
				r.Update(tea.KeyMsg{Type: tea.KeyEsc})
			case "left":
				r.Update(tea.KeyMsg{Type: tea.KeyLeft})
			case "backspace":
				r.Update(tea.KeyMsg{Type: tea.KeyBackspace})
			case "ctrl+u":
				r.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
			default:
				r.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
			}
		}
	}
	// i edits the current value, keys are inserted at the cursor
	k("l", "i")
	if !r.Editing() {
		t.Fatal("i should open the editor")
	}
	k("left", "X", "enter")
	e := r.TakeEdit()
	if e == nil || e.Row != 0 || e.Col != 1 || e.Value != "aXb" {
		t.Fatalf("edit: %+v", e)
	}
	if r.Editing() || r.TakeEdit() != nil {
		t.Fatal("editor should be closed and the edit consumed")
	}
	// the displayed value only changes once the app applies it
	if r.cell(0, 1) != "ab" {
		t.Fatalf("cell changed before apply: %q", r.cell(0, 1))
	}
	r.SetCell(0, 1, "a very long value indeed")
	if r.cell(0, 1) != "a very long value indeed" || r.widths[1] != 24 {
		t.Fatalf("SetCell: %q width=%d", r.cell(0, 1), r.widths[1])
	}

	// esc cancels, movement keys are not interpreted while editing
	k("j", "c")
	if !r.Editing() || string(r.editBuf) != "" {
		t.Fatalf("c should start from an empty value: %q", string(r.editBuf))
	}
	k("j", "q", "esc")
	if r.Editing() || r.TakeEdit() != nil || r.curRow != 1 {
		t.Fatal("esc should cancel without moving")
	}
	// enter also edits; ctrl+u clears; the view shows the editor
	k("enter", "ctrl+u", "x", "y")
	if !strings.Contains(r.View(), "xy") {
		t.Fatal("editor not rendered")
	}
	k("backspace", "enter")
	if e := r.TakeEdit(); e == nil || e.Value != "x" || e.Row != 1 {
		t.Fatalf("edit: %+v", e)
	}

	// CanEdit veto: no editor, the reason goes to Status
	r.CanEdit = func(col int) error { return errors.New("nope") }
	k("i")
	if r.Editing() || r.Status != "nope" {
		t.Fatalf("veto: editing=%v status=%q", r.Editing(), r.Status)
	}
}

func TestAppCellEditKeys(t *testing.T) {
	a := newTestApp()
	a.res.SetData([]string{"id", "v"}, [][]string{{"1", "a"}})
	a.layout()
	press(a, "j", "enter") // focus results
	press(a, "l", "i")
	if !a.res.Editing() || !strings.Contains(a.View(), "INSERT") {
		t.Fatal("results should be in insert mode")
	}
	// q and ? are typed, not interpreted; esc closes the editor but keeps the focus
	press(a, "q", "?")
	if a.confirmQuit || a.showHelp != helpNone || string(a.res.editBuf) != "aq?" {
		t.Fatalf("keys interpreted while editing: %q", string(a.res.editBuf))
	}
	press(a, "esc")
	if a.res.Editing() || !a.focused || a.zone != ZoneResults {
		t.Fatal("esc should only close the editor")
	}
	// committing without a backing table is refused
	press(a, "i", "z", "enter")
	if !a.statusErr || a.res.cell(0, 1) != "a" {
		t.Fatalf("status=%q err=%v cell=%q", a.status, a.statusErr, a.res.cell(0, 1))
	}
	// changing zone cancels the edition
	press(a, "i", "tab")
	if a.res.Editing() {
		t.Fatal("zone change should cancel the edition")
	}
}

func TestConnFormPasswordMaskedAndReturned(t *testing.T) {
	_ = config.SetStoredPassword("kc", "hunter2")
	defer config.DeleteStoredPassword("kc")
	f := NewConnForm(&config.Connection{Name: "kc", Host: "h", Port: 5432, User: "u", PasswordEnv: "LEGACY"})
	f.SetSize(60, 20)
	if v := f.View(); strings.Contains(v, "hunter2") || !strings.Contains(v, "•••••••") {
		t.Fatalf("password must be masked:\n%s", v)
	}
	res := f.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	if res == nil || res.Password != "hunter2" {
		t.Fatalf("password should round-trip, got %+v", res)
	}
	if res.Conn.PasswordEnv != "LEGACY" {
		t.Fatal("password_env must be preserved")
	}
}
