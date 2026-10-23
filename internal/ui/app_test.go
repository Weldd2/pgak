package ui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"pgak/internal/config"
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
	press(a, "enter")   // focus editor
	press(a, "i", "x")  // insert mode
	press(a, "esc")     // insert -> normal, still focused
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
