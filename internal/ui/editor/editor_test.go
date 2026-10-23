package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(e *Editor, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "ctrl+r":
			msg = tea.KeyMsg{Type: tea.KeyCtrlR}
		case "ctrl+n":
			msg = tea.KeyMsg{Type: tea.KeyCtrlN}
		case "ctrl+p":
			msg = tea.KeyMsg{Type: tea.KeyCtrlP}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		e.Update(msg)
	}
}

func typeText(e *Editor, s string) {
	for _, r := range s {
		if r == '\n' {
			key(e, "enter")
		} else {
			key(e, string(r))
		}
	}
}

func TestInsertAndEscape(t *testing.T) {
	e := New()
	key(e, "i")
	if e.Mode() != Insert {
		t.Fatal("expected insert mode")
	}
	typeText(e, "select 1")
	key(e, "esc")
	if e.Mode() != Normal {
		t.Fatal("expected normal mode after esc")
	}
	if e.Value() != "select 1" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestDDAndPaste(t *testing.T) {
	e := New()
	e.SetValue("line1\nline2\nline3")
	key(e, "d", "d")
	if e.Value() != "line2\nline3" {
		t.Fatalf("after dd got %q", e.Value())
	}
	key(e, "p")
	if e.Value() != "line2\nline1\nline3" {
		t.Fatalf("after p got %q", e.Value())
	}
}

func TestYankYY(t *testing.T) {
	e := New()
	e.SetValue("abc\ndef")
	key(e, "y", "y", "j", "p")
	if e.Value() != "abc\ndef\nabc" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestDW(t *testing.T) {
	e := New()
	e.SetValue("hello world")
	key(e, "d", "w")
	if e.Value() != "world" {
		t.Fatalf("after dw got %q", e.Value())
	}
}

func TestMotions(t *testing.T) {
	e := New()
	e.SetValue("foo bar baz")
	key(e, "w")
	if e.col != 4 {
		t.Fatalf("w: col=%d", e.col)
	}
	key(e, "e")
	if e.col != 6 {
		t.Fatalf("e: col=%d", e.col)
	}
	key(e, "b")
	if e.col != 4 {
		t.Fatalf("b: col=%d", e.col)
	}
	key(e, "$")
	if e.col != 10 {
		t.Fatalf("$: col=%d", e.col)
	}
	key(e, "0")
	if e.col != 0 {
		t.Fatalf("0: col=%d", e.col)
	}
}

func TestGGAndG(t *testing.T) {
	e := New()
	e.SetValue("a\nb\nc\nd")
	key(e, "G")
	if e.row != 3 {
		t.Fatalf("G: row=%d", e.row)
	}
	key(e, "g", "g")
	if e.row != 0 {
		t.Fatalf("gg: row=%d", e.row)
	}
	key(e, "2", "G")
	if e.row != 1 {
		t.Fatalf("2G: row=%d", e.row)
	}
}

func TestUndoRedo(t *testing.T) {
	e := New()
	e.SetValue("keep")
	key(e, "d", "d")
	if e.Value() != "" {
		t.Fatalf("dd got %q", e.Value())
	}
	key(e, "u")
	if e.Value() != "keep" {
		t.Fatalf("undo got %q", e.Value())
	}
	key(e, "ctrl+r")
	if e.Value() != "" {
		t.Fatalf("redo got %q", e.Value())
	}
}

func TestVisualYank(t *testing.T) {
	e := New()
	e.SetValue("hello")
	key(e, "v", "l", "l", "y") // select "hel", yank
	key(e, "$", "p")
	if e.Value() != "hellohel" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestVisualLineDelete(t *testing.T) {
	e := New()
	e.SetValue("a\nb\nc")
	key(e, "V", "j", "d")
	if e.Value() != "c" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestOpenLine(t *testing.T) {
	e := New()
	e.SetValue("top")
	key(e, "o")
	typeText(e, "bottom")
	key(e, "esc")
	if e.Value() != "top\nbottom" {
		t.Fatalf("got %q", e.Value())
	}
	key(e, "O")
	typeText(e, "middle")
	key(e, "esc")
	if e.Value() != "top\nmiddle\nbottom" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestCW(t *testing.T) {
	e := New()
	e.SetValue("foo bar")
	key(e, "c", "w")
	if e.Mode() != Insert {
		t.Fatal("cw should enter insert")
	}
	typeText(e, "baz ")
	key(e, "esc")
	if e.Value() != "baz bar" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestCountMotion(t *testing.T) {
	e := New()
	e.SetValue("abcdef")
	key(e, "3", "l")
	if e.col != 3 {
		t.Fatalf("3l: col=%d", e.col)
	}
	key(e, "2", "x")
	if e.Value() != "abcf" {
		t.Fatalf("2x: got %q", e.Value())
	}
}

func TestReplaceChar(t *testing.T) {
	e := New()
	e.SetValue("cat")
	key(e, "r", "b")
	if e.Value() != "bat" {
		t.Fatalf("r: got %q", e.Value())
	}
}

func TestJoin(t *testing.T) {
	e := New()
	e.SetValue("select *\n  from t")
	key(e, "J")
	if e.Value() != "select * from t" {
		t.Fatalf("J: got %q", e.Value())
	}
}

func TestDollarDelete(t *testing.T) {
	e := New()
	e.SetValue("hello world")
	key(e, "w", "D")
	if e.Value() != "hello " {
		t.Fatalf("D: got %q", e.Value())
	}
}

func TestCiw(t *testing.T) {
	e := New()
	e.SetValue("select users from t")
	key(e, "w", "l", "l") // cursor inside "users"
	key(e, "c", "i", "w")
	if e.Mode() != Insert {
		t.Fatal("ciw should enter insert")
	}
	typeText(e, "orders")
	key(e, "esc")
	if e.Value() != "select orders from t" {
		t.Fatalf("ciw: got %q", e.Value())
	}
}

func TestDiwDaw(t *testing.T) {
	e := New()
	e.SetValue("foo bar baz")
	key(e, "w", "d", "i", "w") // delete "bar"
	if e.Value() != "foo  baz" {
		t.Fatalf("diw: got %q", e.Value())
	}
	e.SetValue("foo bar baz")
	key(e, "w", "d", "a", "w") // delete "bar "
	if e.Value() != "foo baz" {
		t.Fatalf("daw: got %q", e.Value())
	}
}

func TestYiwPaste(t *testing.T) {
	e := New()
	e.SetValue("alpha beta")
	key(e, "y", "i", "w") // yank "alpha"
	key(e, "$", "p")
	if e.Value() != "alpha betaalpha" {
		t.Fatalf("yiw+p: got %q", e.Value())
	}
}

func TestQuoteObjects(t *testing.T) {
	e := New()
	e.SetValue(`where name = 'alice' and x`)
	key(e, "f", "a", ";") // cursor on the 'a' of alice (2nd 'a')
	key(e, "c", "i", "'")
	typeText(e, "bob")
	key(e, "esc")
	if e.Value() != `where name = 'bob' and x` {
		t.Fatalf("ci': got %q", e.Value())
	}
	e.SetValue(`say "hello world" now`)
	key(e, "f", "h", "d", "a", `"`)
	if e.Value() != `say  now` {
		t.Fatalf(`da": got %q`, e.Value())
	}
}

func TestParenObjects(t *testing.T) {
	e := New()
	e.SetValue("count(id, name)")
	key(e, "f", "(", "l") // cursor inside parens
	key(e, "d", "i", "(")
	if e.Value() != "count()" {
		t.Fatalf("di(: got %q", e.Value())
	}
	e.SetValue("count(id, name)")
	key(e, "f", "(", "l", "d", "a", "b")
	if e.Value() != "count" {
		t.Fatalf("dab: got %q", e.Value())
	}
}

func TestParenObjectMultiline(t *testing.T) {
	e := New()
	e.SetValue("f(\n  a,\n  b\n)")
	key(e, "j", "c", "i", "(") // from inside, change inner parens
	if e.Mode() != Insert {
		t.Fatal("ci( should enter insert")
	}
	typeText(e, "x")
	key(e, "esc")
	if e.Value() != "f(x)" {
		t.Fatalf("ci( multiline: got %q", e.Value())
	}
}

func TestFindMotions(t *testing.T) {
	e := New()
	e.SetValue("select * from users;")
	key(e, "f", "m")
	if e.col != 12 { // the 'm' of "from"
		t.Fatalf("fm: col=%d", e.col)
	}
	key(e, ";") // no more 'm' → stays
	if e.col != 12 {
		t.Fatalf(";: col=%d", e.col)
	}
	key(e, "f", "u") // 'u' of users
	if e.col != 14 {
		t.Fatalf("fu: col=%d", e.col)
	}
	key(e, ",") // back to previous 'u'... none before except col 14? 'u' not before → stays? there is no earlier u
	key(e, "0", "t", "*")
	if e.col != 6 { // just before '*'
		t.Fatalf("t*: col=%d", e.col)
	}
	key(e, "0", "d", "t", "f") // delete until 'f' of from
	if e.Value() != "from users;" {
		t.Fatalf("dtf: got %q", e.Value())
	}
}

func TestDfInclusive(t *testing.T) {
	e := New()
	e.SetValue("abc,def")
	key(e, "d", "f", ",")
	if e.Value() != "def" {
		t.Fatalf("df,: got %q", e.Value())
	}
}

func TestPercentMotion(t *testing.T) {
	e := New()
	e.SetValue("sum(a + (b))")
	key(e, "%") // first bracket after cursor is '(' at 3 → match at 11
	if e.col != 11 {
		t.Fatalf("%%: col=%d", e.col)
	}
	key(e, "%")
	if e.col != 3 {
		t.Fatalf("%% back: col=%d", e.col)
	}
}

func TestVisualTextObject(t *testing.T) {
	e := New()
	e.SetValue("foo (bar baz) qux")
	key(e, "f", "b")           // cursor on 'b' of bar
	key(e, "v", "i", "(", "y") // viw( yank "bar baz"
	key(e, "g", "g", "0", "P")
	if e.Value() != "bar bazfoo (bar baz) qux" {
		t.Fatalf("vi(y: got %q", e.Value())
	}
}

func newCompEditor() *Editor {
	e := New()
	e.SetSize(60, 10)
	e.SetCompletions([]string{"users", "user_roles", "SELECT", "FROM"})
	return e
}

func TestCompletionAutoOpenAndAccept(t *testing.T) {
	e := newCompEditor()
	key(e, "i")
	typeText(e, "us") // 2 chars: auto-open
	if !e.CompletionActive() {
		t.Fatal("popup should auto-open after 2 chars")
	}
	if len(e.compItems) != 2 {
		t.Fatalf("items=%v", e.compItems)
	}
	key(e, "tab")
	if e.Value() != "users" {
		t.Fatalf("accept: got %q", e.Value())
	}
	if e.CompletionActive() {
		t.Fatal("popup should close after accept")
	}
	if e.Mode() != Insert {
		t.Fatal("accept should stay in insert mode")
	}
}

func TestCompletionNavigateAndEnter(t *testing.T) {
	e := newCompEditor()
	key(e, "i")
	typeText(e, "us")
	key(e, "ctrl+n") // select 2nd item
	key(e, "enter")
	if e.Value() != "user_roles" {
		t.Fatalf("got %q", e.Value())
	}
	// ctrl+p wraps back
	key(e, "esc", "d", "d", "i")
	typeText(e, "us")
	key(e, "ctrl+p", "ctrl+p", "enter") // down 2 with wrap → back to first... (2 items: p,p = same)
	if e.Value() != "users" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestCompletionKeywordCase(t *testing.T) {
	e := newCompEditor()
	key(e, "i")
	typeText(e, "sel")
	if !e.CompletionActive() {
		t.Fatal("popup should be open")
	}
	key(e, "enter")
	if e.Value() != "select" {
		t.Fatalf("lowercase prefix should insert lowercase keyword, got %q", e.Value())
	}
	key(e, "esc", "d", "d", "i")
	typeText(e, "SEL")
	key(e, "enter")
	if e.Value() != "SELECT" {
		t.Fatalf("uppercase prefix keeps the keyword case, got %q", e.Value())
	}
}

func TestCompletionEscClosesAndStaysInsert(t *testing.T) {
	e := newCompEditor()
	key(e, "i")
	typeText(e, "us")
	key(e, "esc")
	if e.CompletionActive() {
		t.Fatal("esc should close the popup")
	}
	if e.Mode() != Insert {
		t.Fatal("first esc only closes the popup")
	}
	key(e, "esc")
	if e.Mode() != Normal {
		t.Fatal("second esc leaves insert mode")
	}
}

func TestCompletionRefilterAndClose(t *testing.T) {
	e := newCompEditor()
	key(e, "i")
	typeText(e, "user_")
	if !e.CompletionActive() || len(e.compItems) != 1 {
		t.Fatalf("expected 1 item, got %v", e.compItems)
	}
	typeText(e, "x") // no match → closes
	if e.CompletionActive() {
		t.Fatal("popup should close when nothing matches")
	}
	key(e, "backspace") // backspace alone must not reopen
	if e.CompletionActive() {
		t.Fatal("popup should stay closed after backspace")
	}
	// exact word fully typed: not proposed
	key(e, "esc", "d", "d", "i")
	typeText(e, "users")
	if e.CompletionActive() {
		t.Fatal("exact match only should not open the popup")
	}
}

func TestCompletionManualCtrlN(t *testing.T) {
	e := newCompEditor()
	key(e, "i")
	typeText(e, "u") // 1 char: no auto-open
	if e.CompletionActive() {
		t.Fatal("popup must not auto-open below 2 chars")
	}
	key(e, "ctrl+n")
	if !e.CompletionActive() {
		t.Fatal("ctrl+n should open the popup manually")
	}
	key(e, "enter")
	if e.Value() != "users" {
		t.Fatalf("got %q", e.Value())
	}
}

func TestCompletionPopupRender(t *testing.T) {
	e := newCompEditor()
	e.SetFocused(true)
	key(e, "i")
	typeText(e, "us")
	v := e.View()
	if !strings.Contains(v, "users") || !strings.Contains(v, "user_roles") {
		t.Fatalf("popup items not rendered:\n%s", v)
	}
}

func TestFindCancelEsc(t *testing.T) {
	e := New()
	e.SetValue("abc")
	key(e, "d", "f", "esc") // cancel df
	if e.Value() != "abc" {
		t.Fatalf("df<esc>: got %q", e.Value())
	}
	if e.PendingKeys() != "" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
}
