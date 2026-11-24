package editor

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var specialKeys = map[string]tea.KeyType{
	"esc": tea.KeyEsc, "enter": tea.KeyEnter, "backspace": tea.KeyBackspace, "tab": tea.KeyTab,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"ctrl+d": tea.KeyCtrlD, "ctrl+u": tea.KeyCtrlU, "ctrl+f": tea.KeyCtrlF, "ctrl+b": tea.KeyCtrlB,
	"ctrl+w": tea.KeyCtrlW, "ctrl+r": tea.KeyCtrlR, "ctrl+h": tea.KeyCtrlH, "home": tea.KeyHome, "end": tea.KeyEnd,
}

// press sends keys; multi-char strings name special keys, single chars are
// typed as runes ("space" types a space).
func press(e *Editor, keys ...string) {
	for _, k := range keys {
		switch {
		case k == "space":
			e.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		case specialKeys[k] != 0:
			e.Update(tea.KeyMsg{Type: specialKeys[k]})
		default:
			for _, r := range k {
				e.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			}
		}
	}
}

func newEd(text string) *Editor {
	e := New()
	e.SetSize(60, 6) // contentHeight = 6 without banner
	e.SetValue(text)
	return e
}

func wantCursor(t *testing.T, e *Editor, row, col int, what string) {
	t.Helper()
	if e.row != row || e.col != col {
		t.Fatalf("%s: cursor=(%d,%d) want (%d,%d)", what, e.row, e.col, row, col)
	}
}

func wantValue(t *testing.T, e *Editor, want, what string) {
	t.Helper()
	if e.Value() != want {
		t.Fatalf("%s: got %q want %q", what, e.Value(), want)
	}
}

func TestBigWordMotions(t *testing.T) {
	e := newEd("foo.bar baz-qux end")
	press(e, "W")
	wantCursor(t, e, 0, 8, "W")
	press(e, "E")
	wantCursor(t, e, 0, 14, "E")
	press(e, "B")
	wantCursor(t, e, 0, 8, "B")
	press(e, "w") // small word stops at the '-'
	wantCursor(t, e, 0, 11, "w")
	press(e, "$", "g", "E")
	wantCursor(t, e, 0, 14, "gE")
	press(e, "g", "e")
	wantCursor(t, e, 0, 11, "ge stops on the '-' (a punctuation word)")
	press(e, "0", "2", "W")
	wantCursor(t, e, 0, 16, "2W")
}

func TestLineMotions(t *testing.T) {
	e := newEd("  one\n    two\nthree  \n\nfive")
	press(e, "enter")
	wantCursor(t, e, 1, 4, "enter -> first non-blank of next line")
	press(e, "+")
	wantCursor(t, e, 2, 0, "+")
	press(e, "-")
	wantCursor(t, e, 1, 4, "-")
	press(e, "2", "_")
	wantCursor(t, e, 2, 0, "2_")
	press(e, "g", "_")
	wantCursor(t, e, 2, 4, "g_ (last non-blank)")
	press(e, "$")
	wantCursor(t, e, 2, 6, "$")
	press(e, "3", "|")
	wantCursor(t, e, 2, 2, "3|")
	press(e, "g", "g", "2", "$")
	wantCursor(t, e, 1, 6, "2$ ends on the next line")
	press(e, "G")
	wantCursor(t, e, 4, 0, "G")
	press(e, "g", "m")
	wantCursor(t, e, 4, 2, "gm")
}

func TestScreenMotionsAndScroll(t *testing.T) {
	e := newEd("l0\nl1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\nl11")
	press(e, "9", "j") // scroll so that row 9 is the last visible: scroll=4
	if e.scroll != 4 {
		t.Fatalf("scroll=%d", e.scroll)
	}
	press(e, "H")
	wantCursor(t, e, 4, 0, "H")
	press(e, "L")
	wantCursor(t, e, 9, 0, "L")
	press(e, "M")
	wantCursor(t, e, 6, 0, "M")
	press(e, "2", "H")
	wantCursor(t, e, 5, 0, "2H")
	press(e, "ctrl+d") // half page = 3
	wantCursor(t, e, 8, 0, "ctrl+d")
	press(e, "ctrl+u")
	wantCursor(t, e, 5, 0, "ctrl+u")
	press(e, "g", "g", "ctrl+f") // page = 4
	wantCursor(t, e, 4, 0, "ctrl+f")
	press(e, "ctrl+b")
	wantCursor(t, e, 0, 0, "ctrl+b")
	press(e, "G", "z", "t")
	if e.scroll != 11 {
		t.Fatalf("zt: scroll=%d", e.scroll)
	}
	press(e, "z", "b")
	if e.scroll != 6 {
		t.Fatalf("zb: scroll=%d", e.scroll)
	}
	press(e, "6", "G", "z", "z")
	if e.scroll != 2 {
		t.Fatalf("zz: scroll=%d", e.scroll)
	}
	press(e, "5", "0", "%")
	wantCursor(t, e, 5, 0, "50%")
}

func TestParagraphAndSentenceMotions(t *testing.T) {
	e := newEd("select a\nfrom t;\n\nselect b. From u! Done\n\n\nend")
	press(e, "}")
	wantCursor(t, e, 2, 0, "}")
	press(e, "}")
	wantCursor(t, e, 4, 0, "} second")
	press(e, "}")
	wantCursor(t, e, 6, 2, "} end of buffer")
	press(e, "{")
	wantCursor(t, e, 5, 0, "{")
	press(e, "{")
	wantCursor(t, e, 2, 0, "{ second")
	press(e, "g", "g", ")")
	wantCursor(t, e, 2, 0, ") stops on the empty line")
	press(e, ")")
	wantCursor(t, e, 3, 0, ") next sentence")
	press(e, ")")
	wantCursor(t, e, 3, 10, ") after the period")
	press(e, ")")
	wantCursor(t, e, 3, 18, ") after the bang")
	press(e, "(")
	wantCursor(t, e, 3, 10, "(")
	press(e, "(", "(")
	wantCursor(t, e, 2, 0, "( back to the empty line")
}

func TestUnmatchedBracketMotions(t *testing.T) {
	e := newEd("f(a, g(b), {c, d})")
	press(e, "f", "b")
	press(e, "[", "(")
	wantCursor(t, e, 0, 6, "[( inner")
	press(e, "[", "(")
	wantCursor(t, e, 0, 1, "[( outer")
	press(e, "f", "c", "]", "}")
	wantCursor(t, e, 0, 16, "]}")
	press(e, "]", ")")
	wantCursor(t, e, 0, 17, "])")
}

func TestMarksAndJumps(t *testing.T) {
	e := newEd("  one\ntwo\nthree\nfour")
	press(e, "j", "l", "m", "a")
	press(e, "G")
	press(e, "'", "a")
	wantCursor(t, e, 1, 0, "'a goes to the first non-blank")
	press(e, "`", "a")
	wantCursor(t, e, 1, 1, "`a goes to the exact column")
	press(e, "G") // jump: '' goes back
	press(e, "'", "'")
	wantCursor(t, e, 1, 0, "''")
	press(e, "`", "`")
	wantCursor(t, e, 3, 0, "`` toggles back")
	press(e, "d", "'", "a") // linewise delete down to the mark
	wantValue(t, e, "  one", "d'a")
}

func TestSearch(t *testing.T) {
	e := newEd("select id\nfrom users\nwhere id = 1 -- Users")
	press(e, "/", "users", "enter")
	wantCursor(t, e, 1, 5, "/users (smartcase: matches Users too)")
	press(e, "n")
	wantCursor(t, e, 2, 16, "n")
	press(e, "n")
	wantCursor(t, e, 1, 5, "n wraps")
	press(e, "N")
	wantCursor(t, e, 2, 16, "N")
	press(e, "/", "Users", "enter")
	wantCursor(t, e, 2, 16, "/Users is case-sensitive")
	press(e, "g", "g", "*")
	wantCursor(t, e, 0, 0, "* on a unique word wraps to itself")
	press(e, "w", "*") // 'id': whole-word search
	wantCursor(t, e, 2, 6, "* id")
	press(e, "#")
	wantCursor(t, e, 0, 7, "#")
	press(e, "?", "from", "enter")
	wantCursor(t, e, 1, 0, "?from (wraps backward)")
	if e.PendingKeys() != "" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "/", "zzz", "enter")
	wantCursor(t, e, 1, 0, "no match does not move")
	press(e, "/", "id", "esc")
	wantCursor(t, e, 1, 0, "esc cancels")
	press(e, "0", "d", "/", "users", "enter")
	wantValue(t, e, "select id\nusers\nwhere id = 1 -- Users", "d/users")
}

func TestSearchPromptPending(t *testing.T) {
	e := newEd("abc")
	press(e, "/", "ab")
	if e.PendingKeys() != "/ab▌" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "backspace", "backspace")
	if e.PendingKeys() != "/▌" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "backspace") // empty prompt + backspace closes it
	if e.searching || e.PendingKeys() != "" {
		t.Fatal("backspace on an empty prompt should cancel")
	}
}

func TestOperatorMotionRules(t *testing.T) {
	// dw on the last word of a line does not join the lines
	e := newEd("foo bar\nbaz")
	press(e, "w", "d", "w")
	wantValue(t, e, "foo \nbaz", "dw last word")
	// dw on an empty line deletes the line (exclusive-linewise rule)
	e = newEd("a\n\nb")
	press(e, "j", "d", "w")
	wantValue(t, e, "a\nb", "dw empty line")
	// d} from the start of a paragraph is linewise
	e = newEd("one\ntwo\n\nthree")
	press(e, "d", "}")
	wantValue(t, e, "\nthree", "d}")
	// d} from inside a line: charwise, stops at the end of the previous line
	e = newEd("one two\n\nthree")
	press(e, "w", "d", "}")
	wantValue(t, e, "one \n\nthree", "d} mid-line")
	// de is inclusive, db exclusive
	e = newEd("alpha beta gamma")
	press(e, "w", "d", "e")
	wantValue(t, e, "alpha  gamma", "de")
	press(e, "$", "d", "b")
	wantValue(t, e, "alpha  a", "db")
	// counts multiply: 2d2w
	e = newEd("a b c d e f")
	press(e, "2", "d", "2", "w")
	wantValue(t, e, "e f", "2d2w")
	// dG / dgg linewise, d% inclusive, dj dk
	e = newEd("1\n2\n3\n4")
	press(e, "j", "d", "G")
	wantValue(t, e, "1", "dG")
	e = newEd("1\n2\n3\n4")
	press(e, "G", "d", "k")
	wantValue(t, e, "1\n2", "dk")
	e = newEd("x(a)y")
	press(e, "l", "d", "%")
	wantValue(t, e, "xy", "d%")
	// d+ d- d_ dH dL linewise
	e = newEd("1\n2\n3")
	press(e, "d", "+")
	wantValue(t, e, "3", "d+")
	// y$ keeps the cursor, yb moves back
	e = newEd("hello world")
	press(e, "w", "y", "b")
	wantCursor(t, e, 0, 0, "yb moves to the start")
	if e.reg.text != "hello " {
		t.Fatalf("reg=%q", e.reg.text)
	}
	// dgE, dge inclusive backward
	e = newEd("foo bar")
	press(e, "$", "d", "g", "e")
	wantValue(t, e, "fo", "dge is inclusive both ends")
}

func TestChangeWordSpecialCases(t *testing.T) {
	e := newEd("foo bar baz")
	press(e, "c", "w")
	press(e, "X", "esc")
	wantValue(t, e, "X bar baz", "cw")
	e = newEd("foo bar baz")
	press(e, "e", "c", "w") // on the last char of the word: changes that char only
	press(e, "Y", "esc")
	wantValue(t, e, "foY bar baz", "cw at word end")
	e = newEd("foo bar baz")
	press(e, "c", "2", "w", "Z", "esc")
	wantValue(t, e, "Z baz", "c2w")
	e = newEd("foo   bar")
	press(e, "e", "l", "c", "w", "-", "esc") // on blanks: cw behaves like dw
	wantValue(t, e, "foo-bar", "cw on blanks")
	e = newEd("a.b c")
	press(e, "c", "W", "x", "esc")
	wantValue(t, e, "x c", "cW")
}

func TestLinewiseChangeKeepsIndent(t *testing.T) {
	e := newEd("select\n    from t\nwhere")
	press(e, "j", "c", "c", "x", "esc")
	wantValue(t, e, "select\n    x\nwhere", "cc keeps indent")
	press(e, "S", "y", "esc")
	wantValue(t, e, "select\n    y\nwhere", "S")
	e = newEd("only")
	press(e, "c", "c", "z", "esc")
	wantValue(t, e, "z", "cc on a single line")
	e = newEd("a\nb")
	press(e, "c", "j", "k", "esc")
	wantValue(t, e, "k", "cj over the whole buffer")
}

func TestIndentOperators(t *testing.T) {
	e := newEd("a\nb\n\nc")
	press(e, ">", ">")
	wantValue(t, e, "    a\nb\n\nc", ">>")
	press(e, "3", ">", ">")
	wantValue(t, e, "        a\n    b\n\nc", "3>> skips the empty line")
	press(e, "<", "G")
	wantValue(t, e, "    a\nb\n\nc", "<G")
	press(e, "<", "<")
	wantValue(t, e, "a\nb\n\nc", "<<")
	press(e, "V", "j", "2", ">")
	wantValue(t, e, "        a\n        b\n\nc", "V 2>")
	wantCursor(t, e, 0, 8, "cursor on the first non-blank after >")
	press(e, "g", "v", "<")
	wantValue(t, e, "    a\n    b\n\nc", "gv <")
	press(e, ">", "j")
	wantValue(t, e, "        a\n        b\n\nc", ">j")
}

func TestCaseOperators(t *testing.T) {
	e := newEd("select Name from T")
	press(e, "g", "U", "i", "w")
	wantValue(t, e, "SELECT Name from T", "gUiw")
	press(e, "g", "u", "u")
	wantValue(t, e, "select name from t", "guu")
	press(e, "w", "g", "~", "e")
	wantValue(t, e, "select NAME from t", "g~e")
	press(e, "g", "~", "~")
	wantValue(t, e, "SELECT name FROM T", "g~~")
	press(e, "0", "3", "~")
	wantValue(t, e, "selECT name FROM T", "3~")
	wantCursor(t, e, 0, 3, "~ moves right")
	press(e, "v", "e", "U")
	wantValue(t, e, "selECT name FROM T", "vU (already upper)")
	press(e, "v", "e", "u")
	wantValue(t, e, "select name FROM T", "vu")
	press(e, "V", "~")
	wantValue(t, e, "SELECT NAME from t", "V~")
	press(e, "g", "U", "G")
	wantValue(t, e, "SELECT NAME FROM T", "gUG")
}

func TestMoreNormalCommands(t *testing.T) {
	e := newEd("abcdef")
	press(e, "$", "2", "X")
	wantValue(t, e, "abcf", "2X")
	wantCursor(t, e, 0, 3, "X cursor")
	press(e, "0", "s", "Z", "esc")
	wantValue(t, e, "Zbcf", "s")
	press(e, "Y", "p")
	wantValue(t, e, "Zbcf\nZbcf", "Y + p")
	press(e, "g", "g", "y", "y", "j", "g", "p")
	wantValue(t, e, "Zbcf\nZbcf\nZbcf", "gp")
	wantCursor(t, e, 2, 0, "gp cursor after the pasted line (clamped)")
	e = newEd("a b")
	press(e, "y", "w", "g", "P")
	wantValue(t, e, "a a b", "gP")
	wantCursor(t, e, 0, 2, "gP cursor after the text")
	press(e, "3", "p")
	wantValue(t, e, "a aa a a  b", "3p")
	e = newEd("one\n  two\nthree\nfour")
	press(e, "3", "J")
	wantValue(t, e, "one two three\nfour", "3J")
	wantCursor(t, e, 0, 7, "J cursor at the join")
	press(e, "g", "J")
	wantValue(t, e, "one two threefour", "gJ")
	e = newEd("f(\n)")
	press(e, "J")
	wantValue(t, e, "f()", "J does not add a space before )")
	e = newEd("x\n\ny")
	press(e, "J")
	wantValue(t, e, "x\ny", "J with an empty next line")
	e = newEd("ab")
	press(e, "r", "space")
	wantValue(t, e, " b", "r space")
	press(e, "0", "3", "r", "x")
	wantValue(t, e, " b", "3r on a short line does nothing")
	press(e, "2", "r", "x")
	wantValue(t, e, "xx", "2rx")
	wantCursor(t, e, 0, 1, "2rx cursor on the last replaced char")
	e = newEd("  ab")
	press(e, "A", "c", "esc", "g", "I", "x", "esc")
	wantValue(t, e, "x  abc", "gI")
	press(e, "g", "i", "d", "esc")
	wantValue(t, e, "xd  abc", "gi resumes where insert mode was left")
	e = newEd("    indented")
	press(e, "o", "x", "esc", "O", "y", "esc")
	wantValue(t, e, "    indented\n    y\n    x", "o/O keep the indent")
}

func TestReplaceMode(t *testing.T) {
	e := newEd("hello")
	press(e, "l", "R", "EY", "esc")
	wantValue(t, e, "hEYlo", "R")
	if e.Mode() != Normal {
		t.Fatal("esc leaves replace mode")
	}
	wantCursor(t, e, 0, 2, "cursor after esc")
	press(e, "0", "R", "abcdefg", "esc")
	wantValue(t, e, "abcdefg", "R past the end appends")
	press(e, "0", "R", "XY", "backspace", "backspace", "Z", "esc")
	wantValue(t, e, "Zbcdefg", "backspace restores the original chars")
	if e.Mode() != Normal {
		t.Fatal("expected normal mode")
	}
}

func TestDotRepeat(t *testing.T) {
	e := newEd("a b c d e f g")
	press(e, "d", "w", ".")
	wantValue(t, e, "c d e f g", "dw .")
	press(e, "2", ".")
	wantValue(t, e, "e f g", "2. replaces the count")
	e = newEd("foo foo foo")
	press(e, "c", "i", "w", "bar", "esc")
	press(e, "w", ".", "w", ".")
	wantValue(t, e, "bar bar bar", "ciw<text> .")
	e = newEd("x")
	press(e, "A", "!", "esc", ".", ".")
	wantValue(t, e, "x!!!", "A .")
	e = newEd("a\nb\nc\nd")
	press(e, "d", "d", "u") // u is not repeatable: . still repeats dd
	press(e, ".")
	wantValue(t, e, "b\nc\nd", "u then .")
	e = newEd("1 2 3 4")
	press(e, "x", "3", ".")
	wantValue(t, e, "3 4", "x then 3.")
	e = newEd("aaa\naaa\naaa")
	press(e, "V", "j", ">", "esc", "G", ".")
	wantValue(t, e, "    aaa\n    aaa\n    aaa", "visual > then . on another line")
	e = newEd("one two")
	press(e, "~", ".", ".")
	wantValue(t, e, "ONE two", "~ .")
	e = newEd("abc")
	press(e, "r", "x", "l", ".")
	wantValue(t, e, "xxc", "rx .")
}

func TestVisualCommands(t *testing.T) {
	e := newEd("hello world")
	press(e, "v", "e", "o", "l") // o swaps anchor/cursor, then l moves the (new) cursor
	a, b, _ := e.selRange()
	if a != (pos{0, 1}) || b != (pos{0, 4}) {
		t.Fatalf("o: sel=%v-%v", a, b)
	}
	press(e, "esc", "0", "v", "e", "r", "-")
	wantValue(t, e, "----- world", "visual r")
	e = newEd("one two")
	press(e, "y", "i", "w", "w", "v", "i", "w", "p")
	wantValue(t, e, "one one", "visual p replaces the selection")
	if e.reg.text != "two" {
		t.Fatalf("register should hold the replaced text, got %q", e.reg.text)
	}
	press(e, "0", "v", "e", "p")
	wantValue(t, e, "two one", "visual p swaps back")
	e = newEd("a\nb\nc")
	press(e, "y", "y", "j", "V", "p")
	wantValue(t, e, "a\na\nc", "V p with a linewise register")
	e = newEd("a\nb\nc")
	press(e, "V", "j", "J")
	wantValue(t, e, "a b\nc", "visual J")
	e = newEd("a\nb\nc")
	press(e, "v", "D")
	wantValue(t, e, "b\nc", "v D is linewise")
	press(e, "v", "Y", "G", "p")
	wantValue(t, e, "b\nc\nb", "v Y is linewise")
	press(e, "g", "g", "v", "C", "z", "esc")
	wantValue(t, e, "z\nc\nb", "v C changes the line")
	e = newEd("select * from t")
	press(e, "v", "$", "y")
	if e.reg.text != "select * from t" {
		t.Fatalf("v$y: reg=%q", e.reg.text)
	}
	press(e, "v", "/", "from", "enter", "d") // the selection is inclusive: the 'f' goes too
	wantValue(t, e, "rom t", "v/from d")
	e = newEd("foo bar")
	press(e, "v", "e", "esc", "$", "g", "v", "d")
	wantValue(t, e, " bar", "gv reselects")
	press(e, "v", "i", "w", "esc")
	if e.PendingKeys() != "" || e.Mode() != Normal {
		t.Fatal("esc leaves visual mode")
	}
}

func TestParagraphAndSentenceObjects(t *testing.T) {
	e := newEd("a\nb\n\n\nc\nd\n\ne")
	press(e, "d", "i", "p")
	wantValue(t, e, "\n\nc\nd\n\ne", "dip")
	e = newEd("a\nb\n\n\nc\nd\n\ne")
	press(e, "d", "a", "p")
	wantValue(t, e, "c\nd\n\ne", "dap takes the blank lines after")
	e = newEd("a\nb\n\n\nc\nd\n\ne")
	press(e, "G", "d", "a", "p")
	wantValue(t, e, "a\nb\n\n\nc\nd", "dap on the last paragraph takes the blank lines before")
	e = newEd("a\nb\n\n\nc\nd")
	press(e, "2", "j", "d", "i", "p")
	wantValue(t, e, "a\nb\nc\nd", "dip on blank lines deletes them")
	e = newEd("a\nb\n\nc")
	press(e, "v", "i", "p", "d")
	wantValue(t, e, "\nc", "vipd")
	if e.Mode() != Normal {
		t.Fatal("expected normal mode")
	}
	e = newEd("One two. Three four!  Five.")
	press(e, "f", "T", "d", "i", "s")
	wantValue(t, e, "One two.   Five.", "dis keeps the trailing blanks")
	e = newEd("One two. Three four!  Five.")
	press(e, "f", "T", "d", "a", "s")
	wantValue(t, e, "One two. Five.", "das")
	e = newEd("One two. Three four!")
	press(e, "$", "d", "a", "s")
	wantValue(t, e, "One two.", "das on the last sentence takes the blank before")
	e = newEd("One two. Three four!")
	press(e, "v", "i", "s", "y")
	if e.reg.text != "One two." {
		t.Fatalf("vis: reg=%q", e.reg.text)
	}
}

func TestWordObjectsBig(t *testing.T) {
	e := newEd("x foo.bar y")
	press(e, "f", "b", "d", "i", "W")
	wantValue(t, e, "x  y", "diW")
	e = newEd("x foo.bar y")
	press(e, "f", "b", "d", "a", "W")
	wantValue(t, e, "x y", "daW")
	e = newEd("x foo.bar y")
	press(e, "f", "b", "d", "i", "w")
	wantValue(t, e, "x foo. y", "diw inside a WORD")
	e = newEd(`a "b"  c`)
	press(e, "f", "b", "c", "a", `"`, "Q", "esc")
	wantValue(t, e, "a Qc", `ca" eats the trailing blanks`)
	e = newEd("f(a, (b))")
	press(e, "$", "d", "i", "(") // cursor on the outer ')': inner of the outer pair
	wantValue(t, e, "f()", "di( on the closing bracket")
	e = newEd("f(a, (b))")
	press(e, "f", "(", "d", "a", "(")
	wantValue(t, e, "f", "da( on the opening bracket")
	e = newEd("[1, [2]]")
	press(e, "f", "2", "d", "a", "[")
	wantValue(t, e, "[1, ]", "da[")
	e = newEd("<a <b>>")
	press(e, "f", "b", "c", "i", "<", "x", "esc")
	wantValue(t, e, "<a <x>>", "ci<")
	e = newEd("{ {x} }")
	press(e, "f", "x", "d", "a", "B")
	wantValue(t, e, "{  }", "daB")
}

func TestInsertModeEditingKeys(t *testing.T) {
	e := newEd("")
	press(e, "i", "select foo bar", "ctrl+w")
	wantValue(t, e, "select foo ", "ctrl+w")
	press(e, "ctrl+w", "ctrl+w")
	wantValue(t, e, "", "ctrl+w twice")
	press(e, "abc def", "ctrl+u")
	wantValue(t, e, "", "ctrl+u")
	press(e, "xy", "home", "0", "end", "9", "esc")
	wantValue(t, e, "0xy9", "home/end")
	press(e, "0", "i", "ctrl+h", "esc") // ctrl+h at col 0 does nothing
	wantValue(t, e, "0xy9", "ctrl+h at start")
}

func TestPendingKeysDisplay(t *testing.T) {
	e := newEd("abc")
	press(e, "2", "d", "3")
	if e.PendingKeys() != "2d3" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "esc")
	if e.PendingKeys() != "" {
		t.Fatalf("esc should clear, got %q", e.PendingKeys())
	}
	press(e, "g")
	if e.PendingKeys() != "g" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "esc", "d", "i")
	if e.PendingKeys() != "di" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "esc", "m")
	if e.PendingKeys() != "m" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "esc")
	wantValue(t, e, "abc", "nothing changed")
}

func TestTextObjectCountAndInvalid(t *testing.T) {
	e := newEd("abc")
	press(e, "d", "i", "z") // unknown object aborts the operator
	wantValue(t, e, "abc", "diz")
	if e.PendingKeys() != "" {
		t.Fatalf("pending=%q", e.PendingKeys())
	}
	press(e, "d", "q") // unknown motion aborts
	wantValue(t, e, "abc", "dq")
	press(e, "d", "n") // n without a previous search aborts
	wantValue(t, e, "abc", "dn")
	press(e, "d", "'", "z") // unset mark aborts
	wantValue(t, e, "abc", "d'z")
	if e.PendingKeys() != "" || e.Mode() != Normal {
		t.Fatal("editor should be idle")
	}
}

func TestSpaceAndBackspaceMotions(t *testing.T) {
	e := newEd("abc")
	press(e, "space", "space")
	wantCursor(t, e, 0, 2, "space")
	press(e, "backspace")
	wantCursor(t, e, 0, 1, "backspace")
	press(e, "d", "space")
	wantValue(t, e, "ac", "d<space>")
}
