// Package editor implements a small vim-like text editor component for
// Bubble Tea: normal / insert / visual / visual-line modes, counts,
// operators (d, y, c) with motions, undo/redo and system clipboard yank.
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
)

func (m Mode) String() string {
	switch m {
	case Insert:
		return "INSERT"
	case Visual:
		return "VISUAL"
	case VisualLine:
		return "V-LINE"
	default:
		return "NORMAL"
	}
}

type snapshot struct {
	lines    [][]rune
	row, col int
}

type register struct {
	text     string
	linewise bool
}

type Editor struct {
	lines [][]rune
	row   int
	col   int
	want  int // desired column for j/k

	mode     Mode
	pending  rune    // pending operator: d, y, c (or r)
	gPend    bool    // pending 'g' (for gg)
	objPend  rune    // pending text object: 'i' or 'a' (after an operator or in visual)
	findPend rune    // pending find command: f F t T (waiting for target char)
	lastFind [2]rune // last find command + char, for ; and ,
	count    int     // count prefix (0 = none)

	vrow, vcol int // visual anchor

	reg   register
	undos []snapshot
	redos []snapshot

	scroll  int
	width   int
	height  int
	focused bool

	banner string // dim, non-editable line shown above the buffer

	cursorStyle lipgloss.Style
	selStyle    lipgloss.Style
	numStyle    lipgloss.Style
	bannerStyle lipgloss.Style
}

func New() *Editor {
	return &Editor{
		lines:       [][]rune{{}},
		cursorStyle: lipgloss.NewStyle().Reverse(true),
		selStyle:    lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")),
		numStyle:    lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		bannerStyle: lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true),
	}
}

func (e *Editor) SetSize(w, h int) { e.width, e.height = w, h }
func (e *Editor) SetFocused(b bool) {
	e.focused = b
	if !b && e.mode == Insert {
		// leaving the zone keeps insert mode; that's fine
	}
}
func (e *Editor) Mode() Mode { return e.mode }

// SetBanner sets a dim, non-editable line displayed above the buffer
// (empty string hides it).
func (e *Editor) SetBanner(s string) { e.banner = s }

// PendingKeys returns a short string describing pending operator/count, for
// display in a status bar.
func (e *Editor) PendingKeys() string {
	s := ""
	if e.count > 0 {
		s += itoa(e.count)
	}
	if e.pending != 0 {
		s += string(e.pending)
	}
	if e.gPend {
		s += "g"
	}
	if e.objPend != 0 {
		s += string(e.objPend)
	}
	if e.findPend != 0 {
		s += string(e.findPend)
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
	e.row, e.col, e.scroll = 0, 0, 0
	e.mode = Normal
	e.pending, e.gPend, e.count = 0, false, 0
	e.objPend, e.findPend = 0, 0
	e.undos, e.redos = nil, nil
}

// ---------------------------------------------------------------- helpers

func (e *Editor) line() []rune { return e.lines[e.row] }

func (e *Editor) maxCol() int {
	n := len(e.line())
	if e.mode == Insert {
		return n
	}
	if n == 0 {
		return 0
	}
	return n - 1
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

func cloneLines(lines [][]rune) [][]rune {
	out := make([][]rune, len(lines))
	for i, l := range lines {
		out[i] = append([]rune(nil), l...)
	}
	return out
}

func (e *Editor) pushUndo() {
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

// ------------------------------------------------------------- word logic

func classOf(r rune) int {
	switch {
	case r == ' ' || r == '\t':
		return 0
	case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_':
		return 1
	default:
		return 2
	}
}

// nextWord returns position of start of next word (vim 'w').
func (e *Editor) nextWord(row, col int) (int, int) {
	line := e.lines[row]
	if col < len(line) {
		cl := classOf(line[col])
		for col < len(line) && classOf(line[col]) == cl {
			col++
		}
		for col < len(line) && classOf(line[col]) == 0 {
			col++
		}
		if col < len(line) {
			return row, col
		}
	}
	// wrap to next lines
	for r := row + 1; r < len(e.lines); r++ {
		l := e.lines[r]
		c := 0
		for c < len(l) && classOf(l[c]) == 0 {
			c++
		}
		if c < len(l) || len(l) == 0 {
			return r, c
		}
	}
	last := len(e.lines) - 1
	return last, maxInt(0, len(e.lines[last]))
}

// endWord returns position of end of word (vim 'e').
func (e *Editor) endWord(row, col int) (int, int) {
	for {
		line := e.lines[row]
		col++
		for col < len(line) && classOf(line[col]) == 0 {
			col++
		}
		if col < len(line) {
			cl := classOf(line[col])
			for col+1 < len(line) && classOf(line[col+1]) == cl {
				col++
			}
			return row, col
		}
		if row+1 >= len(e.lines) {
			return row, maxInt(0, len(line)-1)
		}
		row++
		col = -1
	}
}

// prevWord returns position of start of previous word (vim 'b').
func (e *Editor) prevWord(row, col int) (int, int) {
	for {
		line := e.lines[row]
		col--
		for col >= 0 && classOf(line[col]) == 0 {
			col--
		}
		if col >= 0 {
			cl := classOf(line[col])
			for col-1 >= 0 && classOf(line[col-1]) == cl {
				col--
			}
			return row, col
		}
		if row == 0 {
			return 0, 0
		}
		row--
		col = len(e.lines[row])
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ------------------------------------------------------------ text ranges

type pos struct{ row, col int }

func lessPos(a, b pos) bool {
	return a.row < b.row || (a.row == b.row && a.col < b.col)
}

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

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------- update

// Update handles a key message. Keys not consumed (zone switching, etc.) are
// expected to be filtered by the caller beforehand.
func (e *Editor) Update(msg tea.KeyMsg) {
	switch e.mode {
	case Insert:
		e.updateInsert(msg)
	case Visual, VisualLine:
		e.updateVisual(msg)
	default:
		e.updateNormal(msg)
	}
	e.ensureVisible()
}

func (e *Editor) updateInsert(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc":
		e.mode = Normal
		if e.col > 0 {
			e.col--
		}
		e.clamp()
	case "enter":
		line := e.line()
		rest := append([]rune(nil), line[e.col:]...)
		e.lines[e.row] = append([]rune(nil), line[:e.col]...)
		e.lines = append(e.lines[:e.row+1], append([][]rune{rest}, e.lines[e.row+1:]...)...)
		e.row++
		e.col = 0
	case "backspace":
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
	case "tab":
		e.insertRunes([]rune("    "))
	case "left":
		if e.col > 0 {
			e.col--
		}
	case "right":
		if e.col < len(e.line()) {
			e.col++
		}
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
				line := e.line()
				rest := append([]rune(nil), line[e.col:]...)
				e.lines[e.row] = append([]rune(nil), line[:e.col]...)
				e.lines = append(e.lines[:e.row+1], append([][]rune{rest}, e.lines[e.row+1:]...)...)
				e.row++
				e.col = 0
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

func (e *Editor) takeCount() int {
	n := e.count
	e.count = 0
	if n == 0 {
		return 1
	}
	return n
}

func (e *Editor) updateNormal(msg tea.KeyMsg) {
	key := msg.String()

	// pending find target (f/F/t/T — the next key is the target char)
	if e.findPend != 0 {
		e.handleFindTarget(key)
		return
	}

	// count prefix
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		if !(key == "0" && e.count == 0) { // bare 0 is a motion
			e.count = e.count*10 + int(key[0]-'0')
			return
		}
	}

	// pending g
	if e.gPend {
		e.gPend = false
		if key == "g" {
			target := clampInt(e.takeCount()-1, 0, len(e.lines)-1)
			if e.pending != 0 {
				e.applyLinewiseOp(e.row, target)
			} else {
				e.row, e.col = target, 0
				e.clamp()
			}
		}
		return
	}

	// pending operator
	if e.pending != 0 {
		e.handleOperatorMotion(key)
		return
	}

	switch key {
	case "esc":
		e.count = 0
	case "i":
		e.pushUndo()
		e.mode = Insert
	case "a":
		e.pushUndo()
		e.mode = Insert
		if len(e.line()) > 0 {
			e.col++
		}
	case "I":
		e.pushUndo()
		e.mode = Insert
		e.col = firstNonBlank(e.line())
	case "A":
		e.pushUndo()
		e.mode = Insert
		e.col = len(e.line())
	case "o":
		e.pushUndo()
		e.mode = Insert
		e.lines = append(e.lines[:e.row+1], append([][]rune{{}}, e.lines[e.row+1:]...)...)
		e.row++
		e.col = 0
	case "O":
		e.pushUndo()
		e.mode = Insert
		e.lines = append(e.lines[:e.row], append([][]rune{{}}, e.lines[e.row:]...)...)
		e.col = 0
	case "v":
		e.mode = Visual
		e.vrow, e.vcol = e.row, e.col
	case "V":
		e.mode = VisualLine
		e.vrow, e.vcol = e.row, e.col
	case "h", "left":
		e.col -= e.takeCount()
		e.clamp()
		e.want = e.col
	case "l", "right":
		e.col += e.takeCount()
		e.clamp()
		e.want = e.col
	case "j", "down":
		e.row = clampInt(e.row+e.takeCount(), 0, len(e.lines)-1)
		e.col = clampInt(e.want, 0, e.maxCol())
	case "k", "up":
		e.row = clampInt(e.row-e.takeCount(), 0, len(e.lines)-1)
		e.col = clampInt(e.want, 0, e.maxCol())
	case "0":
		e.col = 0
		e.want = 0
	case "^":
		e.col = firstNonBlank(e.line())
		e.want = e.col
	case "$":
		e.col = e.maxCol()
		e.want = 1 << 30
	case "w":
		for i := 0; i < e.takeCount(); i++ {
			e.row, e.col = e.nextWord(e.row, e.col)
		}
		e.clamp()
		e.want = e.col
	case "b":
		for i := 0; i < e.takeCount(); i++ {
			e.row, e.col = e.prevWord(e.row, e.col)
		}
		e.clamp()
		e.want = e.col
	case "e":
		for i := 0; i < e.takeCount(); i++ {
			e.row, e.col = e.endWord(e.row, e.col)
		}
		e.clamp()
		e.want = e.col
	case "g":
		e.gPend = true
	case "G":
		if e.count > 0 {
			e.row = clampInt(e.takeCount()-1, 0, len(e.lines)-1)
		} else {
			e.row = len(e.lines) - 1
		}
		e.col = 0
		e.clamp()
	case "x":
		e.pushUndo()
		n := e.takeCount()
		line := e.line()
		end := clampInt(e.col+n, 0, len(line))
		if e.col < len(line) {
			e.setReg(string(line[e.col:end]), false)
			e.lines[e.row] = append(append([]rune(nil), line[:e.col]...), line[end:]...)
			e.clamp()
		}
	case "D":
		e.pushUndo()
		line := e.line()
		if e.col < len(line) {
			e.setReg(string(line[e.col:]), false)
			e.lines[e.row] = append([]rune(nil), line[:e.col]...)
			e.clamp()
		}
	case "C":
		e.pushUndo()
		line := e.line()
		if e.col <= len(line) {
			e.setReg(string(line[e.col:]), false)
			e.lines[e.row] = append([]rune(nil), line[:e.col]...)
		}
		e.mode = Insert
	case "s":
		e.pushUndo()
		line := e.line()
		if e.col < len(line) {
			e.setReg(string(line[e.col:e.col+1]), false)
			e.lines[e.row] = append(append([]rune(nil), line[:e.col]...), line[e.col+1:]...)
		}
		e.mode = Insert
	case "d":
		e.pending = 'd'
	case "y":
		e.pending = 'y'
	case "c":
		e.pending = 'c'
	case "p":
		e.pushUndo()
		e.paste(true)
	case "P":
		e.pushUndo()
		e.paste(false)
	case "u":
		e.undo()
	case "ctrl+r":
		e.redo()
	case "f", "F", "t", "T":
		e.findPend = rune(key[0])
	case ";", ",":
		if e.lastFind[0] != 0 {
			cmd := e.lastFind[0]
			if key == "," {
				cmd = reverseFind(cmd)
			}
			e.doFind(cmd, e.lastFind[1])
		}
	case "%":
		if m, ok := e.matchBracket(); ok {
			e.row, e.col = m.row, m.col
			e.clamp()
			e.want = e.col
		}
	case "J":
		e.pushUndo()
		if e.row < len(e.lines)-1 {
			cur := e.line()
			next := e.lines[e.row+1]
			joined := append(append([]rune(nil), cur...), ' ')
			joined = append(joined, []rune(strings.TrimLeft(string(next), " \t"))...)
			e.col = len(cur)
			e.lines[e.row] = joined
			e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
		}
	case "r":
		e.pending = 'r'
	default:
		// 'r' replacement char arrives as a rune; handled via pending above
	}
}

func firstNonBlank(line []rune) int {
	for i, r := range line {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return 0
}

func (e *Editor) setReg(text string, linewise bool) {
	e.reg = register{text, linewise}
	e.yankToClipboard(text)
}

func (e *Editor) paste(after bool) {
	if e.reg.text == "" {
		return
	}
	if e.reg.linewise {
		newLines := strings.Split(e.reg.text, "\n")
		insert := make([][]rune, len(newLines))
		for i, l := range newLines {
			insert[i] = []rune(l)
		}
		at := e.row
		if after {
			at++
		}
		e.lines = append(e.lines[:at], append(insert, e.lines[at:]...)...)
		e.row = at
		e.col = 0
		e.clamp()
		return
	}
	col := e.col
	if after && len(e.line()) > 0 {
		col++
	}
	if strings.Contains(e.reg.text, "\n") {
		parts := strings.Split(e.reg.text, "\n")
		line := e.line()
		head := append([]rune(nil), line[:clampInt(col, 0, len(line))]...)
		tail := append([]rune(nil), line[clampInt(col, 0, len(line)):]...)
		block := make([][]rune, len(parts))
		for i, p := range parts {
			block[i] = []rune(p)
		}
		block[0] = append(head, block[0]...)
		lastIdx := len(block) - 1
		endCol := len(block[lastIdx])
		block[lastIdx] = append(block[lastIdx], tail...)
		e.lines = append(e.lines[:e.row], append(block, e.lines[e.row+1:]...)...)
		e.row += lastIdx
		e.col = maxInt(0, endCol-1)
		e.clamp()
		return
	}
	line := e.line()
	col = clampInt(col, 0, len(line))
	rs := []rune(e.reg.text)
	nl := make([]rune, 0, len(line)+len(rs))
	nl = append(nl, line[:col]...)
	nl = append(nl, rs...)
	nl = append(nl, line[col:]...)
	e.lines[e.row] = nl
	e.col = col + len(rs) - 1
	e.clamp()
}

func (e *Editor) applyLinewiseOp(from, to int) {
	op := e.pending
	e.pending = 0
	if from > to {
		from, to = to, from
	}
	switch op {
	case 'y':
		e.setReg(e.linesText(from, to), true)
		e.row = from
		e.clamp()
	case 'd':
		e.pushUndo()
		e.setReg(e.linesText(from, to), true)
		e.deleteLines(from, to)
	case 'c':
		e.pushUndo()
		e.setReg(e.linesText(from, to), true)
		e.deleteLines(from, to)
		e.lines = append(e.lines[:e.row], append([][]rune{{}}, e.lines[e.row:]...)...)
		e.col = 0
		e.mode = Insert
	}
}

func (e *Editor) applyCharwiseOp(a, b pos) {
	op := e.pending
	e.pending = 0
	text := e.extract(a, b)
	switch op {
	case 'y':
		e.setReg(text, false)
		if lessPos(b, a) {
			a, b = b, a
		}
		e.row, e.col = a.row, a.col
		e.clamp()
	case 'd':
		e.pushUndo()
		e.setReg(text, false)
		e.deleteRange(a, b)
	case 'c':
		e.pushUndo()
		e.setReg(text, false)
		e.deleteRange(a, b)
		e.mode = Insert
	}
}

func (e *Editor) handleOperatorMotion(key string) {
	op := e.pending

	// replace char
	if op == 'r' {
		e.pending = 0
		if len(key) == 1 {
			line := e.line()
			if e.col < len(line) {
				e.pushUndo()
				line[e.col] = []rune(key)[0]
			}
		}
		return
	}

	// pending text object (the key after di/da/ci/ca/yi/ya)
	if e.objPend != 0 {
		around := e.objPend == 'a'
		e.objPend = 0
		if a, b, ok := e.textObject(key, around); ok {
			e.applyCharwiseOp(a, b)
		} else {
			e.pending = 0
		}
		e.count = 0
		return
	}

	switch key {
	case "esc":
		e.pending = 0
		e.count = 0
	case string(op): // dd, yy, cc
		n := e.takeCount()
		e.applyLinewiseOp(e.row, e.row+n-1)
	case "w":
		n := e.takeCount()
		r, c := e.row, e.col
		for i := 0; i < n; i++ {
			r, c = e.nextWord(r, c)
		}
		// dw on same line should not eat the newline; clamp to line end
		if op != 'y' && r > e.row {
			r, c = e.row, len(e.line())
		}
		e.applyCharwiseOp(pos{e.row, e.col}, pos{r, c})
	case "e":
		n := e.takeCount()
		r, c := e.row, e.col
		for i := 0; i < n; i++ {
			r, c = e.endWord(r, c)
		}
		e.applyCharwiseOp(pos{e.row, e.col}, pos{r, c + 1})
	case "b":
		n := e.takeCount()
		r, c := e.row, e.col
		for i := 0; i < n; i++ {
			r, c = e.prevWord(r, c)
		}
		e.applyCharwiseOp(pos{r, c}, pos{e.row, e.col})
	case "$":
		e.applyCharwiseOp(pos{e.row, e.col}, pos{e.row, len(e.line())})
	case "0":
		e.applyCharwiseOp(pos{e.row, 0}, pos{e.row, e.col})
	case "^":
		e.applyCharwiseOp(pos{e.row, firstNonBlank(e.line())}, pos{e.row, e.col})
	case "h":
		n := e.takeCount()
		e.applyCharwiseOp(pos{e.row, maxInt(0, e.col-n)}, pos{e.row, e.col})
	case "l":
		n := e.takeCount()
		e.applyCharwiseOp(pos{e.row, e.col}, pos{e.row, clampInt(e.col+n, 0, len(e.line()))})
	case "j":
		n := e.takeCount()
		e.applyLinewiseOp(e.row, clampInt(e.row+n, 0, len(e.lines)-1))
	case "k":
		n := e.takeCount()
		e.applyLinewiseOp(clampInt(e.row-n, 0, len(e.lines)-1), e.row)
	case "G":
		e.applyLinewiseOp(e.row, len(e.lines)-1)
	case "g":
		e.gPend = true
		return
	case "i", "a": // text object follows (iw, a", i(, …)
		e.objPend = rune(key[0])
		return
	case "f", "F", "t", "T":
		e.findPend = rune(key[0])
		return
	case ";", ",":
		if e.lastFind[0] == 0 {
			e.pending = 0
			e.count = 0
			return
		}
		cmd := e.lastFind[0]
		if key == "," {
			cmd = reverseFind(cmd)
		}
		e.doFind(cmd, e.lastFind[1])
	case "%":
		if m, ok := e.matchBracket(); ok {
			a, b := pos{e.row, e.col}, m
			if lessPos(b, a) {
				a, b = b, a
			}
			b.col++ // % is inclusive
			e.applyCharwiseOp(a, b)
		} else {
			e.pending = 0
		}
	default:
		if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
			e.count = e.count*10 + int(key[0]-'0')
			return
		}
		e.pending = 0
		e.count = 0
	}
}

// ------------------------------------------------------- find (f/F/t/T ; ,)

func reverseFind(cmd rune) rune {
	switch cmd {
	case 'f':
		return 'F'
	case 'F':
		return 'f'
	case 't':
		return 'T'
	case 'T':
		return 't'
	}
	return cmd
}

// findOnLine returns the target column for a find command, or ok=false.
func findOnLine(line []rune, col int, cmd, ch rune) (int, bool) {
	switch cmd {
	case 'f':
		for c := col + 1; c < len(line); c++ {
			if line[c] == ch {
				return c, true
			}
		}
	case 'F':
		for c := col - 1; c >= 0; c-- {
			if line[c] == ch {
				return c, true
			}
		}
	case 't':
		for c := col + 1; c < len(line); c++ {
			if line[c] == ch {
				return c - 1, true
			}
		}
	case 'T':
		for c := col - 1; c >= 0; c-- {
			if line[c] == ch {
				return c + 1, true
			}
		}
	}
	return 0, false
}

// handleFindTarget consumes the char following f/F/t/T.
func (e *Editor) handleFindTarget(key string) {
	cmd := e.findPend
	e.findPend = 0
	rs := []rune(key)
	if key == "esc" || len(rs) != 1 {
		e.pending = 0
		e.count = 0
		return
	}
	if key == "space" {
		rs = []rune{' '}
	}
	e.lastFind = [2]rune{cmd, rs[0]}
	e.doFind(cmd, rs[0])
}

// doFind moves to the target (or applies the pending operator up to it).
func (e *Editor) doFind(cmd, ch rune) {
	n := e.takeCount()
	col := e.col
	ok := true
	for i := 0; i < n && ok; i++ {
		col, ok = findOnLine(e.line(), col, cmd, ch)
	}
	if !ok {
		e.pending = 0
		return
	}
	if e.pending != 0 {
		if cmd == 'f' || cmd == 't' { // forward, target inclusive
			e.applyCharwiseOp(pos{e.row, e.col}, pos{e.row, col + 1})
		} else { // backward, cursor exclusive
			e.applyCharwiseOp(pos{e.row, col}, pos{e.row, e.col})
		}
		return
	}
	e.col = col
	e.clamp()
	e.want = e.col
}

// ------------------------------------------------------------ text objects

// textObject resolves iw/aw, i"/a" (and ' `), i(/a( (and { [ <) into a
// charwise range (end exclusive).
func (e *Editor) textObject(key string, around bool) (pos, pos, bool) {
	switch key {
	case "w", "W":
		return e.wordObject(key == "W", around)
	case `"`, "'", "`":
		return e.quoteObject(rune(key[0]), around)
	case "(", ")", "b":
		return e.pairObject('(', ')', around)
	case "{", "}", "B":
		return e.pairObject('{', '}', around)
	case "[", "]":
		return e.pairObject('[', ']', around)
	case "<", ">":
		return e.pairObject('<', '>', around)
	}
	return pos{}, pos{}, false
}

// wordObject implements iw/aw (and iW/aW for WORDs).
func (e *Editor) wordObject(big, around bool) (pos, pos, bool) {
	line := e.line()
	if len(line) == 0 {
		return pos{e.row, 0}, pos{e.row, 0}, true
	}
	cls := func(r rune) int {
		c := classOf(r)
		if big && c == 2 {
			c = 1 // a WORD is any run of non-blanks
		}
		return c
	}
	col := clampInt(e.col, 0, len(line)-1)
	cl := cls(line[col])
	start, end := col, col+1
	for start > 0 && cls(line[start-1]) == cl {
		start--
	}
	for end < len(line) && cls(line[end]) == cl {
		end++
	}
	if around && cl != 0 {
		// aw: include trailing whitespace, else leading whitespace
		e0 := end
		for end < len(line) && cls(line[end]) == 0 {
			end++
		}
		if end == e0 {
			for start > 0 && cls(line[start-1]) == 0 {
				start--
			}
		}
	}
	return pos{e.row, start}, pos{e.row, end}, true
}

// quoteObject implements i"/a" (and ' `), on the current line.
func (e *Editor) quoteObject(q rune, around bool) (pos, pos, bool) {
	line := e.line()
	var idx []int
	for i, r := range line {
		if r == q {
			idx = append(idx, i)
		}
	}
	for i := 0; i+1 < len(idx); i += 2 {
		a, b := idx[i], idx[i+1]
		if e.col <= b {
			if around {
				return pos{e.row, a}, pos{e.row, b + 1}, true
			}
			return pos{e.row, a + 1}, pos{e.row, b}, true
		}
	}
	return pos{}, pos{}, false
}

// charAt returns the rune at p ('\n' at end of line).
func (e *Editor) charAt(p pos) rune {
	line := e.lines[p.row]
	if p.col < 0 || p.col >= len(line) {
		return '\n'
	}
	return line[p.col]
}

func (e *Editor) prevPos(p pos) (pos, bool) {
	if p.col > 0 {
		return pos{p.row, p.col - 1}, true
	}
	if p.row == 0 {
		return p, false
	}
	return pos{p.row - 1, len(e.lines[p.row-1])}, true
}

func (e *Editor) nextPos(p pos) (pos, bool) {
	if p.col < len(e.lines[p.row]) {
		return pos{p.row, p.col + 1}, true
	}
	if p.row >= len(e.lines)-1 {
		return p, false
	}
	return pos{p.row + 1, 0}, true
}

// pairObject implements i(/a( and friends, searching the enclosing pair
// (multi-line, nesting-aware).
func (e *Editor) pairObject(open, close rune, around bool) (pos, pos, bool) {
	cur := pos{e.row, clampInt(e.col, 0, maxInt(0, len(e.line())))}

	// backward scan for the opening bracket (a close bracket under the
	// cursor belongs to the pair we are looking for, so it is not counted)
	openP := pos{-1, -1}
	depth := 0
	p := cur
	for {
		switch e.charAt(p) {
		case open:
			if depth == 0 {
				openP = p
			} else {
				depth--
			}
		case close:
			if p != cur {
				depth++
			}
		}
		if openP.row >= 0 {
			break
		}
		var ok bool
		if p, ok = e.prevPos(p); !ok {
			return pos{}, pos{}, false
		}
	}

	// forward scan for the matching closing bracket
	closeP := pos{-1, -1}
	depth = 0
	p, ok := e.nextPos(openP)
	for ok {
		switch e.charAt(p) {
		case open:
			depth++
		case close:
			if depth == 0 {
				closeP = p
			} else {
				depth--
			}
		}
		if closeP.row >= 0 {
			break
		}
		p, ok = e.nextPos(p)
	}
	if closeP.row < 0 {
		return pos{}, pos{}, false
	}
	if around {
		return openP, pos{closeP.row, closeP.col + 1}, true
	}
	return pos{openP.row, openP.col + 1}, closeP, true
}

// matchBracket implements %: jump to the bracket matching the first bracket
// at or after the cursor on the current line.
func (e *Editor) matchBracket() (pos, bool) {
	const opens, closes = "([{", ")]}"
	line := e.line()
	c := e.col
	for c < len(line) && !strings.ContainsRune(opens+closes, line[c]) {
		c++
	}
	if c >= len(line) {
		return pos{}, false
	}
	start := pos{e.row, c}
	if i := strings.IndexRune(opens, line[c]); i >= 0 {
		open, close := line[c], rune(closes[i])
		depth := 0
		p, ok := e.nextPos(start)
		for ok {
			switch e.charAt(p) {
			case close:
				if depth == 0 {
					return p, true
				}
				depth--
			case open:
				depth++
			}
			p, ok = e.nextPos(p)
		}
		return pos{}, false
	}
	i := strings.IndexRune(closes, line[c])
	open, close := rune(opens[i]), line[c]
	depth := 0
	p, ok := e.prevPos(start)
	for ok {
		switch e.charAt(p) {
		case open:
			if depth == 0 {
				return p, true
			}
			depth--
		case close:
			depth++
		}
		p, ok = e.prevPos(p)
	}
	return pos{}, false
}

func (e *Editor) updateVisual(msg tea.KeyMsg) {
	key := msg.String()

	// pending find target
	if e.findPend != 0 {
		e.handleFindTarget(key)
		return
	}
	// pending text object: adjust the selection (viw, va", …)
	if e.objPend != 0 {
		around := e.objPend == 'a'
		e.objPend = 0
		if a, b, ok := e.textObject(key, around); ok && lessPos(a, b) {
			e.vrow, e.vcol = a.row, a.col
			// b is exclusive, the visual end is inclusive
			if b.col == 0 && b.row > a.row {
				e.row = b.row - 1
				e.col = maxInt(0, len(e.lines[e.row])-1)
			} else {
				e.row, e.col = b.row, maxInt(0, b.col-1)
			}
		}
		return
	}

	// count prefix
	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' {
		e.count = e.count*10 + int(key[0]-'0')
		return
	}
	if e.gPend {
		e.gPend = false
		if key == "g" {
			e.row, e.col = 0, 0
		}
		return
	}

	switch key {
	case "esc":
		e.mode = Normal
		e.count = 0
	case "v":
		if e.mode == Visual {
			e.mode = Normal
		} else {
			e.mode = Visual
		}
	case "V":
		if e.mode == VisualLine {
			e.mode = Normal
		} else {
			e.mode = VisualLine
		}
	case "h", "left":
		e.col -= e.takeCount()
		e.clamp()
		e.want = e.col
	case "l", "right":
		e.col += e.takeCount()
		e.clamp()
		e.want = e.col
	case "j", "down":
		e.row = clampInt(e.row+e.takeCount(), 0, len(e.lines)-1)
		e.col = clampInt(e.want, 0, e.maxCol())
	case "k", "up":
		e.row = clampInt(e.row-e.takeCount(), 0, len(e.lines)-1)
		e.col = clampInt(e.want, 0, e.maxCol())
	case "0":
		e.col = 0
	case "^":
		e.col = firstNonBlank(e.line())
	case "$":
		e.col = e.maxCol()
	case "w":
		e.row, e.col = e.nextWord(e.row, e.col)
		e.clamp()
	case "b":
		e.row, e.col = e.prevWord(e.row, e.col)
		e.clamp()
	case "e":
		e.row, e.col = e.endWord(e.row, e.col)
		e.clamp()
	case "g":
		e.gPend = true
	case "G":
		e.row = len(e.lines) - 1
		e.clamp()
	case "i", "a":
		e.objPend = rune(key[0])
	case "f", "F", "t", "T":
		e.findPend = rune(key[0])
	case ";", ",":
		if e.lastFind[0] != 0 {
			cmd := e.lastFind[0]
			if key == "," {
				cmd = reverseFind(cmd)
			}
			e.doFind(cmd, e.lastFind[1])
		}
	case "%":
		if m, ok := e.matchBracket(); ok {
			e.row, e.col = m.row, m.col
			e.clamp()
		}
	case "y":
		e.visualOp('y')
	case "d", "x":
		e.visualOp('d')
	case "c", "s":
		e.visualOp('c')
	}
}

func (e *Editor) visualOp(op rune) {
	linewise := e.mode == VisualLine
	a := pos{e.vrow, e.vcol}
	b := pos{e.row, e.col}
	e.mode = Normal
	e.pending = op
	if linewise {
		e.applyLinewiseOp(a.row, b.row)
		return
	}
	if lessPos(b, a) {
		a, b = b, a
	}
	b.col++ // visual selection is inclusive
	e.applyCharwiseOp(a, b)
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
	return sb.String()
}

func padLeft(s string, w int) string {
	for len(s) < w {
		s = " " + s
	}
	return s
}
