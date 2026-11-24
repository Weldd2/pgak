// Package editor implements a vim-like text editor component for Bubble Tea:
// normal / insert / replace / visual / visual-line modes, counts, operators
// (d y c > < g~ gu gU) combined with motions and text objects, marks,
// search, dot-repeat, undo/redo and system clipboard yank.
//
// The motions live in motion.go and are shared by the three places vim uses
// them: plain cursor movement, operator-pending mode and visual mode.
package editor

import (
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Mode int

const (
	Normal Mode = iota
	Insert
	Visual
	VisualLine
	Replace
)

func (m Mode) String() string {
	switch m {
	case Insert:
		return "INSERT"
	case Replace:
		return "REPLACE"
	case Visual:
		return "VISUAL"
	case VisualLine:
		return "V-LINE"
	default:
		return "NORMAL"
	}
}

const shiftWidth = 4 // columns inserted by > and tab

type snapshot struct {
	lines    [][]rune
	row, col int
}

type register struct {
	text     string
	linewise bool
}

type pos struct{ row, col int }

func lessPos(a, b pos) bool {
	return a.row < b.row || (a.row == b.row && a.col < b.col)
}

// visualSel remembers the last visual selection for gv.
type visualSel struct {
	anchor, cursor pos
	mode           Mode
	ok             bool
}

type Editor struct {
	lines [][]rune
	row   int
	col   int
	want  int // desired column for j/k

	mode Mode

	// pending multi-key state (normal / visual modes)
	op       string // pending operator: d y c > < g~ gu gU ("" = none)
	opCount  int    // count typed before the operator (0 = none)
	count    int    // count being typed (0 = none)
	prefix   string // pending prefix key waiting for its second key: g z [ ]
	charPend rune   // command waiting for a char argument: f F t T r m ' `
	objPend  rune   // 'i' or 'a' waiting for the text object key

	lastFind [2]rune // last f/F/t/T command + char, for ; and ,

	// search (/ ? n N * #)
	searching     bool
	searchInput   string
	searchDir     int // +1 forward (/), -1 backward (?)
	lastSearch    string
	lastSearchDir int
	lastWholeWord bool

	marks    map[rune]pos // m{a-z}
	jumpMark pos          // '' and ``: position before the last jump
	hasJump  bool

	vrow, vcol int       // visual anchor
	lastVis    visualSel // for gv
	lastInsert pos       // where insert mode was last left (gi)

	replaced []rune // replace mode: original chars, -1 = appended (backspace)

	// dot repeat: keys of the last buffer-changing command
	curKeys    []tea.KeyMsg
	lastChange []tea.KeyMsg
	changed    bool
	replaying  bool
	isDot      bool

	reg   register
	undos []snapshot
	redos []snapshot

	scroll  int
	width   int
	height  int
	focused bool

	banner string // dim, non-editable line shown above the buffer

	completions []string // candidate words for insert-mode completion
	compActive  bool     // completion popup open
	compItems   []string // filtered candidates
	compSel     int      // selected candidate

	cursorStyle   lipgloss.Style
	selStyle      lipgloss.Style
	numStyle      lipgloss.Style
	bannerStyle   lipgloss.Style
	popupStyle    lipgloss.Style
	popupSelStyle lipgloss.Style
}

func New() *Editor {
	return &Editor{
		lines:       [][]rune{{}},
		marks:       map[rune]pos{},
		cursorStyle: lipgloss.NewStyle().Reverse(true),
		selStyle:    lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")),
		numStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		bannerStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
		popupStyle:  lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252")),
		popupSelStyle: lipgloss.NewStyle().Background(lipgloss.Color("24")).
			Foreground(lipgloss.Color("231")).Bold(true),
	}
}

func (e *Editor) SetSize(w, h int) { e.width, e.height = w, h }
func (e *Editor) SetFocused(b bool) {
	e.focused = b
}
func (e *Editor) Mode() Mode { return e.mode }

// Typing reports whether keys are inserted as text (insert or replace mode),
// i.e. the host must not intercept printable keys or tab.
func (e *Editor) Typing() bool { return e.mode == Insert || e.mode == Replace }

// SetBanner sets a dim, non-editable line displayed above the buffer
// (empty string hides it).
func (e *Editor) SetBanner(s string) { e.banner = s }

// PendingKeys returns a short string describing pending operator/count, for
// display in a status bar.
func (e *Editor) PendingKeys() string {
	if e.searching {
		d := "/"
		if e.searchDir < 0 {
			d = "?"
		}
		return d + e.searchInput + "▌"
	}
	s := ""
	if e.opCount > 0 {
		s += itoa(e.opCount)
	}
	s += e.op
	if e.count > 0 {
		s += itoa(e.count)
	}
	s += e.prefix
	if e.objPend != 0 {
		s += string(e.objPend)
	}
	if e.charPend != 0 {
		s += string(e.charPend)
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func (e *Editor) Value() string {
	parts := make([]string, len(e.lines))
	for i, l := range e.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func (e *Editor) SetValue(s string) {
	raw := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	e.lines = make([][]rune, len(raw))
	for i, l := range raw {
		e.lines[i] = []rune(l)
	}
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.row, e.col, e.scroll, e.want = 0, 0, 0, 0
	e.mode = Normal
	e.resetPending()
	e.searching = false
	e.undos, e.redos = nil, nil
	e.marks = map[rune]pos{}
	e.hasJump = false
	e.lastVis = visualSel{}
	e.closeCompletion()
}

// resetPending clears every multi-key state (esc, invalid key…).
func (e *Editor) resetPending() {
	e.op, e.opCount, e.count = "", 0, 0
	e.prefix, e.charPend, e.objPend = "", 0, 0
}

// idle reports whether no multi-key command is in progress (normal mode,
// nothing pending): the boundary used to record dot-repeatable changes.
func (e *Editor) idle() bool {
	return e.mode == Normal && e.op == "" && e.count == 0 && e.prefix == "" &&
		e.charPend == 0 && e.objPend == 0 && !e.searching
}

// ---------------------------------------------------------------- helpers

func (e *Editor) line() []rune { return e.lines[e.row] }

func (e *Editor) maxCol() int {
	n := len(e.line())
	if e.mode == Insert || e.mode == Replace {
		return n
	}
	if n == 0 {
		return 0
	}
	return n - 1
}

// maxColAt is the last cursor column of a row in normal mode.
func (e *Editor) maxColAt(row int) int {
	return maxInt(0, len(e.lines[row])-1)
}

func (e *Editor) clamp() {
	if e.row < 0 {
		e.row = 0
	}
	if e.row >= len(e.lines) {
		e.row = len(e.lines) - 1
	}
	if e.col < 0 {
		e.col = 0
	}
	if e.col > e.maxCol() {
		e.col = e.maxCol()
	}
}

// clampPos clamps a position into the buffer (normal-mode columns).
func (e *Editor) clampPos(p pos) pos {
	p.row = clampInt(p.row, 0, len(e.lines)-1)
	p.col = clampInt(p.col, 0, e.maxColAt(p.row))
	return p
}

func cloneLines(lines [][]rune) [][]rune {
	out := make([][]rune, len(lines))
	for i, l := range lines {
		out[i] = append([]rune(nil), l...)
	}
	return out
}

func (e *Editor) pushUndo() {
	e.changed = true
	e.undos = append(e.undos, snapshot{cloneLines(e.lines), e.row, e.col})
	if len(e.undos) > 200 {
		e.undos = e.undos[1:]
	}
	e.redos = nil
}

func (e *Editor) undo() {
	if len(e.undos) == 0 {
		return
	}
	e.redos = append(e.redos, snapshot{cloneLines(e.lines), e.row, e.col})
	s := e.undos[len(e.undos)-1]
	e.undos = e.undos[:len(e.undos)-1]
	e.lines, e.row, e.col = s.lines, s.row, s.col
	e.clamp()
}

func (e *Editor) redo() {
	if len(e.redos) == 0 {
		return
	}
	e.undos = append(e.undos, snapshot{cloneLines(e.lines), e.row, e.col})
	s := e.redos[len(e.redos)-1]
	e.redos = e.redos[:len(e.redos)-1]
	e.lines, e.row, e.col = s.lines, s.row, s.col
	e.clamp()
}

func (e *Editor) yankToClipboard(text string) {
	_ = clipboard.WriteAll(text)
}

func (e *Editor) setReg(text string, linewise bool) {
	e.reg = register{text, linewise}
	e.yankToClipboard(text)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func firstNonBlank(line []rune) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return 0
}

// lastNonBlank returns the column of the last non-blank char (0 if none).
func lastNonBlank(line []rune) int {
	for i := len(line) - 1; i >= 0; i-- {
		if line[i] != ' ' && line[i] != '\t' {
			return i
		}
	}
	return 0
}

func isBlankLine(line []rune) bool {
	for _, r := range line {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}

// ------------------------------------------------------------ text ranges

// extract returns text between a and b (charwise, end exclusive).
func (e *Editor) extract(a, b pos) string {
	if lessPos(b, a) {
		a, b = b, a
	}
	if a.row == b.row {
		line := e.lines[a.row]
		return string(line[clampInt(a.col, 0, len(line)):clampInt(b.col, 0, len(line))])
	}
	var sb strings.Builder
	first := e.lines[a.row]
	sb.WriteString(string(first[clampInt(a.col, 0, len(first)):]))
	sb.WriteByte('\n')
	for r := a.row + 1; r < b.row; r++ {
		sb.WriteString(string(e.lines[r]))
		sb.WriteByte('\n')
	}
	last := e.lines[b.row]
	sb.WriteString(string(last[:clampInt(b.col, 0, len(last))]))
	return sb.String()
}

// deleteRange removes charwise range (end exclusive) and moves cursor to a.
func (e *Editor) deleteRange(a, b pos) {
	if lessPos(b, a) {
		a, b = b, a
	}
	if a.row == b.row {
		line := e.lines[a.row]
		ac := clampInt(a.col, 0, len(line))
		bc := clampInt(b.col, 0, len(line))
		e.lines[a.row] = append(append([]rune{}, line[:ac]...), line[bc:]...)
	} else {
		first := e.lines[a.row]
		last := e.lines[b.row]
		ac := clampInt(a.col, 0, len(first))
		bc := clampInt(b.col, 0, len(last))
		merged := append(append([]rune{}, first[:ac]...), last[bc:]...)
		e.lines = append(e.lines[:a.row], append([][]rune{merged}, e.lines[b.row+1:]...)...)
	}
	e.row, e.col = a.row, a.col
	e.clamp()
}

func (e *Editor) deleteLines(from, to int) {
	from = clampInt(from, 0, len(e.lines)-1)
	to = clampInt(to, 0, len(e.lines)-1)
	if from > to {
		from, to = to, from
	}
	e.lines = append(e.lines[:from], e.lines[to+1:]...)
	if len(e.lines) == 0 {
		e.lines = [][]rune{{}}
	}
	e.row = clampInt(from, 0, len(e.lines)-1)
	e.col = 0
	e.clamp()
}

func (e *Editor) linesText(from, to int) string {
	from = clampInt(from, 0, len(e.lines)-1)
	to = clampInt(to, 0, len(e.lines)-1)
	if from > to {
		from, to = to, from
	}
	parts := make([]string, 0, to-from+1)
	for r := from; r <= to; r++ {
		parts = append(parts, string(e.lines[r]))
	}
	return strings.Join(parts, "\n")
}

// splitLine breaks the current line at the cursor (insert-mode enter).
func (e *Editor) splitLine() {
	line := e.line()
	c := clampInt(e.col, 0, len(line))
	rest := append([]rune(nil), line[c:]...)
	e.lines[e.row] = append([]rune(nil), line[:c]...)
	e.lines = append(e.lines[:e.row+1], append([][]rune{rest}, e.lines[e.row+1:]...)...)
	e.row++
	e.col = 0
}

// ---------------------------------------------------------------- update

// keyString normalizes a key message (space is always " ").
func keyString(msg tea.KeyMsg) string {
	if msg.Type == tea.KeySpace {
		return " "
	}
	return msg.String()
}

// Update handles a key message. Keys not consumed (zone switching, etc.) are
// expected to be filtered by the caller beforehand.
func (e *Editor) Update(msg tea.KeyMsg) {
	rec := !e.replaying
	if rec && e.idle() {
		e.curKeys = e.curKeys[:0]
		e.changed = false
		e.isDot = false
	}
	if rec {
		e.curKeys = append(e.curKeys, msg)
	}

	switch e.mode {
	case Insert:
		e.updateInsert(msg)
	case Replace:
		e.updateReplace(msg)
	case Visual, VisualLine:
		e.updateVisual(msg)
	default:
		e.updateNormal(msg)
	}
	e.ensureVisible()

	// a change is complete once we are back to an idle normal mode
	if rec && e.idle() && e.changed && !e.isDot {
		e.lastChange = append([]tea.KeyMsg(nil), e.curKeys...)
	}
}

// repeatLastChange implements "." (with an optional new count).
func (e *Editor) repeatLastChange(count int) {
	e.isDot = true
	if len(e.lastChange) == 0 {
		return
	}
	keys := e.lastChange
	if count > 0 {
		// a count given to . replaces the count of the original command
		i := 0
		for i < len(keys) && keys[i].Type == tea.KeyRunes && len(keys[i].Runes) == 1 &&
			keys[i].Runes[0] >= '0' && keys[i].Runes[0] <= '9' {
			i++
		}
		var withCount []tea.KeyMsg
		for _, d := range itoa(count) {
			withCount = append(withCount, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{d}})
		}
		keys = append(withCount, keys[i:]...)
	}
	e.replaying = true
	for _, k := range keys {
		e.Update(k)
	}
	e.replaying = false
	// the replay may have left insert mode etc.; make sure we are idle
	if e.mode == Insert || e.mode == Replace {
		e.leaveInsert()
	}
	if e.mode == Visual || e.mode == VisualLine {
		e.leaveVisual()
	}
	e.resetPending()
}

// ------------------------------------------------------------------ insert

// leaveInsert goes back to normal mode (esc in insert/replace mode).
func (e *Editor) leaveInsert() {
	e.lastInsert = pos{e.row, e.col}
	e.mode = Normal
	e.replaced = nil
	e.closeCompletion()
	if e.col > 0 {
		e.col--
	}
	e.clamp()
	e.want = e.col
}

func (e *Editor) updateInsert(msg tea.KeyMsg) {
	key := keyString(msg)

	// completion popup consumes its navigation/validation keys
	if e.compActive && e.handleCompletionKey(key) {
		return
	}
	if key == "ctrl+n" { // manual trigger
		e.openCompletion(1)
		return
	}

	switch key {
	case "esc":
		e.leaveInsert()
	case "enter":
		e.splitLine()
	case "backspace", "ctrl+h":
		if e.col > 0 {
			line := e.line()
			e.lines[e.row] = append(append([]rune(nil), line[:e.col-1]...), line[e.col:]...)
			e.col--
		} else if e.row > 0 {
			prev := e.lines[e.row-1]
			e.col = len(prev)
			e.lines[e.row-1] = append(prev, e.line()...)
			e.lines = append(e.lines[:e.row], e.lines[e.row+1:]...)
			e.row--
		}
		e.refreshCompletion()
	case "delete":
		line := e.line()
		if e.col < len(line) {
			e.lines[e.row] = append(append([]rune(nil), line[:e.col]...), line[e.col+1:]...)
		} else if e.row < len(e.lines)-1 {
			e.lines[e.row] = append(line, e.lines[e.row+1]...)
			e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
		}
	case "ctrl+w": // delete the word before the cursor
		e.closeCompletion()
		line := e.line()
		c := e.col
		for c > 0 && classOf(line[c-1]) == 0 {
			c--
		}
		if c > 0 {
			cl := classOf(line[c-1])
			for c > 0 && classOf(line[c-1]) == cl {
				c--
			}
		}
		e.lines[e.row] = append(append([]rune(nil), line[:c]...), line[e.col:]...)
		e.col = c
	case "ctrl+u": // delete to the start of the line
		e.closeCompletion()
		line := e.line()
		e.lines[e.row] = append([]rune(nil), line[e.col:]...)
		e.col = 0
	case "tab":
		e.insertRunes([]rune(strings.Repeat(" ", shiftWidth)))
	case "left":
		e.closeCompletion()
		if e.col > 0 {
			e.col--
		}
	case "right":
		e.closeCompletion()
		if e.col < len(e.line()) {
			e.col++
		}
	case "home":
		e.closeCompletion()
		e.col = 0
	case "end":
		e.closeCompletion()
		e.col = len(e.line())
	case "up":
		if e.row > 0 {
			e.row--
			e.clamp()
		}
	case "down":
		if e.row < len(e.lines)-1 {
			e.row++
			e.clamp()
		}
	default:
		if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
			runes := msg.Runes
			if msg.Type == tea.KeySpace {
				runes = []rune{' '}
			}
			e.insertRunes(runes)
			// auto-open (or refresh) the completion popup while typing
			if e.compActive {
				e.refreshCompletion()
			} else if len(runes) == 1 && classOf(runes[0]) == 1 {
				e.openCompletion(compAutoMinChars)
			}
		}
	}
}

func (e *Editor) insertRunes(rs []rune) {
	// handle pasted text with newlines
	text := string(rs)
	if strings.ContainsAny(text, "\n\r") {
		text = strings.ReplaceAll(text, "\r\n", "\n")
		text = strings.ReplaceAll(text, "\r", "\n")
		for i, part := range strings.Split(text, "\n") {
			if i > 0 {
				e.splitLine()
			}
			e.insertPlain([]rune(part))
		}
		return
	}
	e.insertPlain(rs)
}

func (e *Editor) insertPlain(rs []rune) {
	line := e.line()
	nl := make([]rune, 0, len(line)+len(rs))
	nl = append(nl, line[:e.col]...)
	nl = append(nl, rs...)
	nl = append(nl, line[e.col:]...)
	e.lines[e.row] = nl
	e.col += len(rs)
}

// ----------------------------------------------------------------- replace

// updateReplace handles replace mode (R): typed chars overwrite the line,
// backspace restores the original chars.
func (e *Editor) updateReplace(msg tea.KeyMsg) {
	key := keyString(msg)
	switch key {
	case "esc":
		e.leaveInsert()
	case "enter":
		e.splitLine()
		e.replaced = nil
	case "backspace", "ctrl+h":
		if len(e.replaced) == 0 {
			if e.col > 0 {
				e.col--
			}
			return
		}
		orig := e.replaced[len(e.replaced)-1]
		e.replaced = e.replaced[:len(e.replaced)-1]
		e.col--
		line := e.line()
		if orig < 0 {
			e.lines[e.row] = append(append([]rune(nil), line[:e.col]...), line[e.col+1:]...)
		} else {
			line[e.col] = orig
		}
	case "left", "right", "up", "down", "home", "end":
		e.replaced = nil
		e.updateInsert(msg)
	default:
		if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
			return
		}
		runes := msg.Runes
		if msg.Type == tea.KeySpace {
			runes = []rune{' '}
		}
		for _, r := range runes {
			if r == '\n' || r == '\r' {
				e.splitLine()
				e.replaced = nil
				continue
			}
			line := e.line()
			if e.col < len(line) {
				e.replaced = append(e.replaced, line[e.col])
				line[e.col] = r
			} else {
				e.replaced = append(e.replaced, -1)
				e.lines[e.row] = append(line, r)
			}
			e.col++
		}
	}
}

// ---------------------------------------------------------------- view

// contentHeight is the number of buffer lines visible (banner excluded).
func (e *Editor) contentHeight() int {
	h := e.height
	if e.banner != "" {
		h--
	}
	if h < 1 {
		h = 1
	}
	return h
}

func (e *Editor) ensureVisible() {
	if e.height <= 0 {
		return
	}
	h := e.contentHeight()
	if e.row < e.scroll {
		e.scroll = e.row
	}
	if e.row >= e.scroll+h {
		e.scroll = e.row - h + 1
	}
	if e.scroll < 0 {
		e.scroll = 0
	}
}

func (e *Editor) selRange() (pos, pos, bool) {
	if e.mode != Visual && e.mode != VisualLine {
		return pos{}, pos{}, false
	}
	a := pos{e.vrow, e.vcol}
	b := pos{e.row, e.col}
	if lessPos(b, a) {
		a, b = b, a
	}
	return a, b, true
}

func (e *Editor) inSelection(r, c int) bool {
	a, b, ok := e.selRange()
	if !ok {
		return false
	}
	if e.mode == VisualLine {
		return r >= a.row && r <= b.row
	}
	if r < a.row || r > b.row {
		return false
	}
	if r == a.row && c < a.col {
		return false
	}
	if r == b.row && c > b.col {
		return false
	}
	return true
}

func (e *Editor) View() string {
	e.ensureVisible()
	numW := 4
	textW := maxInt(1, e.width-numW-1)

	var sb strings.Builder
	if e.banner != "" {
		b := e.banner
		if len([]rune(b)) > e.width {
			b = string([]rune(b)[:maxInt(0, e.width-1)]) + "…"
		}
		sb.WriteString(e.bannerStyle.Render(b))
		sb.WriteByte('\n')
	}
	for i := 0; i < e.contentHeight(); i++ {
		r := e.scroll + i
		if i > 0 {
			sb.WriteByte('\n')
		}
		if r >= len(e.lines) {
			sb.WriteString(e.numStyle.Render(padLeft("~", numW)))
			continue
		}
		sb.WriteString(e.numStyle.Render(padLeft(itoa(r+1), numW)))
		sb.WriteByte(' ')

		line := e.lines[r]
		// horizontal window
		start := 0
		if r == e.row && e.col >= textW {
			start = e.col - textW + 1
		}
		end := clampInt(start+textW, 0, len(line))
		start = clampInt(start, 0, len(line))

		for c := start; c < end; c++ {
			ch := string(line[c])
			if ch == "\t" {
				ch = " "
			}
			switch {
			case e.focused && r == e.row && c == e.col:
				sb.WriteString(e.cursorStyle.Render(ch))
			case e.inSelection(r, c):
				sb.WriteString(e.selStyle.Render(ch))
			default:
				sb.WriteString(ch)
			}
		}
		// cursor at end of line (empty line or insert position)
		if e.focused && r == e.row && e.col >= end && e.col >= len(line) {
			sb.WriteString(e.cursorStyle.Render(" "))
		}
	}
	out := sb.String()
	if e.compActive && e.focused && e.mode == Insert {
		out = e.overlayCompletion(out)
	}
	return out
}

func padLeft(s string, w int) string {
	for len(s) < w {
		s = " " + s
	}
	return s
}
