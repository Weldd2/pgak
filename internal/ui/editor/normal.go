package editor

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
)

// preKey handles the states shared by normal and visual mode: the search
// prompt, commands waiting for a char argument, pending text objects, the
// count prefix and the prefix keys (g z [ ]). It returns the key (composed
// with its prefix if any) and whether it was fully consumed.
func (e *Editor) preKey(msg tea.KeyMsg) (string, bool) {
	key := keyString(msg)
	if e.searching {
		e.updateSearchPrompt(msg)
		return key, true
	}
	if e.charPend != 0 {
		e.handleCharArg(msg)
		return key, true
	}
	if e.objPend != 0 {
		e.handleTextObjectKey(key)
		return key, true
	}
	// count prefix (a bare 0 is the "first column" motion)
	if e.prefix == "" && len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
		if !(key == "0" && e.count == 0) {
			e.count = e.count*10 + int(key[0]-'0')
			return key, true
		}
	}
	if e.prefix != "" {
		if key == "esc" {
			e.resetPending()
			return key, true
		}
		key = e.prefix + key
		e.prefix = ""
		return key, false
	}
	switch key {
	case "g", "z", "[", "]":
		e.prefix = key
		return key, true
	case "f", "F", "t", "T", "m", "'", "`":
		e.charPend = rune(key[0])
		return key, true
	}
	return key, false
}

// effectiveCount combines the count before the operator and the count
// before the motion (2d3w deletes 6 words). 0 = no count.
func (e *Editor) effectiveCount() int {
	switch {
	case e.opCount == 0:
		return e.count
	case e.count == 0:
		return e.opCount
	}
	return e.opCount * e.count
}

// finishMotion applies a resolved motion: operator if one is pending,
// otherwise a plain cursor move. A failed motion aborts the operator.
func (e *Editor) finishMotion(m motion, ok bool) {
	if !ok {
		e.resetPending()
		return
	}
	if e.op != "" {
		e.applyOperator(m)
		return
	}
	e.count, e.opCount = 0, 0
	e.moveTo(m)
}

// handleCharArg consumes the char following f F t T r m ' `.
func (e *Editor) handleCharArg(msg tea.KeyMsg) {
	cmd := e.charPend
	e.charPend = 0
	key := keyString(msg)
	rs := []rune(key)
	if key == "esc" || (msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace) || len(rs) != 1 {
		e.resetPending()
		return
	}
	ch := rs[0]
	n := maxInt(1, e.effectiveCount())
	switch cmd {
	case 'f', 'F', 't', 'T':
		e.lastFind = [2]rune{cmd, ch}
		m, ok := e.findMotion(cmd, ch, n, false)
		e.finishMotion(m, ok)
	case 'r':
		e.replaceChars(ch, n)
	case 'm':
		if unicode.IsLetter(ch) {
			e.marks[ch] = pos{e.row, e.col}
		}
		e.resetPending()
	case '\'', '`':
		m, ok := e.markMotion(cmd, ch)
		e.finishMotion(m, ok)
	}
}

// handleTextObjectKey consumes the key after i/a (diw, va", cip…).
func (e *Editor) handleTextObjectKey(key string) {
	around := e.objPend == 'a'
	e.objPend = 0
	a, b, linewise, ok := e.textObject(key, around)
	if !ok {
		e.resetPending()
		return
	}
	if e.mode == Visual || e.mode == VisualLine {
		e.count = 0
		e.visualSelectObject(a, b, linewise)
		return
	}
	if linewise {
		e.applyLinewiseOp(a.row, b.row)
		return
	}
	e.applyCharwiseOp(a, b)
}

// ----------------------------------------------------------------- normal

func (e *Editor) updateNormal(msg tea.KeyMsg) {
	key, done := e.preKey(msg)
	if done {
		return
	}
	if key == "esc" {
		e.resetPending()
		return
	}
	if e.op != "" {
		e.operatorKey(key)
		return
	}
	if key == "r" {
		e.charPend = 'r'
		return
	}
	if m, ok := e.resolveMotion(key, e.count); ok {
		e.count = 0
		e.moveTo(m)
		return
	}
	e.normalCommand(key)
}

// normalCommand runs a non-motion normal-mode command.
func (e *Editor) normalCommand(key string) {
	n := e.count
	e.count = 0
	n1 := maxInt(1, n)
	last := len(e.lines) - 1

	switch key {
	// ---- entering insert mode
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
	case "gI":
		e.pushUndo()
		e.mode = Insert
		e.col = 0
	case "A":
		e.pushUndo()
		e.mode = Insert
		e.col = len(e.line())
	case "gi":
		e.pushUndo()
		e.mode = Insert
		e.row = clampInt(e.lastInsert.row, 0, last)
		e.col = clampInt(e.lastInsert.col, 0, len(e.line()))
	case "o":
		e.pushUndo()
		e.mode = Insert
		indent := leadingBlanks(e.line())
		e.lines = append(e.lines[:e.row+1], append([][]rune{append([]rune(nil), indent...)}, e.lines[e.row+1:]...)...)
		e.row++
		e.col = len(indent)
	case "O":
		e.pushUndo()
		e.mode = Insert
		indent := leadingBlanks(e.line())
		e.lines = append(e.lines[:e.row], append([][]rune{append([]rune(nil), indent...)}, e.lines[e.row:]...)...)
		e.col = len(indent)
	case "R":
		e.pushUndo()
		e.mode = Replace
		e.replaced = nil

	// ---- visual
	case "v":
		e.mode = Visual
		e.vrow, e.vcol = e.row, e.col
	case "V":
		e.mode = VisualLine
		e.vrow, e.vcol = e.row, e.col
	case "gv":
		if e.lastVis.ok {
			a := e.clampPos(e.lastVis.anchor)
			b := e.clampPos(e.lastVis.cursor)
			e.mode = e.lastVis.mode
			e.vrow, e.vcol = a.row, a.col
			e.row, e.col = b.row, b.col
		}

	// ---- operators
	case "d", "y", "c", ">", "<", "g~", "gu", "gU":
		e.op = key
		e.opCount = n
	case "D":
		e.op, e.count = "d", n
		e.operatorKey("$")
	case "C":
		e.op, e.count = "c", n
		e.operatorKey("$")
	case "Y":
		e.op, e.count = "y", n
		e.operatorKey("y")
	case "S":
		e.op, e.count = "c", n
		e.operatorKey("c")
	case "s":
		e.op, e.count = "c", n
		e.operatorKey("l")

	// ---- simple changes
	case "x", "delete":
		line := e.line()
		if e.col < len(line) {
			e.pushUndo()
			end := clampInt(e.col+n1, 0, len(line))
			e.setReg(string(line[e.col:end]), false)
			e.lines[e.row] = append(append([]rune(nil), line[:e.col]...), line[end:]...)
			e.clamp()
		}
	case "X":
		if e.col > 0 {
			e.pushUndo()
			line := e.line()
			start := maxInt(0, e.col-n1)
			e.setReg(string(line[start:e.col]), false)
			e.lines[e.row] = append(append([]rune(nil), line[:start]...), line[e.col:]...)
			e.col = start
			e.clamp()
		}
	case "~":
		line := e.line()
		if e.col < len(line) {
			e.pushUndo()
			end := clampInt(e.col+n1, 0, len(line))
			for c := e.col; c < end; c++ {
				line[c] = toggleCase(line[c])
			}
			e.col = end
			e.clamp()
		}
	case "J", "gJ":
		e.pushUndo()
		e.joinLines(maxInt(1, n1-1), key == "J")
	case "p", "P", "gp", "gP":
		e.pushUndo()
		e.paste(key == "p" || key == "gp", strings.HasPrefix(key, "g"), n1)
	case "u":
		for i := 0; i < n1; i++ {
			e.undo()
		}
	case "ctrl+r":
		for i := 0; i < n1; i++ {
			e.redo()
		}
	case ".":
		e.repeatLastChange(n)

	// ---- search prompt
	case "/", "?":
		e.searching = true
		e.searchInput = ""
		e.searchDir = 1
		if key == "?" {
			e.searchDir = -1
		}

	// ---- scrolling (z)
	case "zt", "zenter":
		e.scroll = e.row
		if key == "zenter" {
			e.col = firstNonBlank(e.line())
		}
	case "zz", "z.":
		e.scroll = maxInt(0, e.row-e.contentHeight()/2)
		if key == "z." {
			e.col = firstNonBlank(e.line())
		}
	case "zb", "z-":
		e.scroll = maxInt(0, e.row-e.contentHeight()+1)
		if key == "z-" {
			e.col = firstNonBlank(e.line())
		}
	}
}

// operatorKey handles the key following a pending operator.
func (e *Editor) operatorKey(key string) {
	op := e.op
	last := len(e.lines) - 1

	// doubled operator works on lines: dd yy cc >> << g~~ guu gUU (g~g~ gugu gUgU)
	if key == op || (len(op) == 2 && key == op[1:]) {
		n := maxInt(1, e.effectiveCount())
		e.applyLinewiseOp(e.row, clampInt(e.row+n-1, 0, last))
		return
	}
	switch key {
	case "esc":
		e.resetPending()
		return
	case "i", "a": // text object follows
		e.objPend = rune(key[0])
		return
	case "/", "?": // d/foo<CR>
		e.searching = true
		e.searchInput = ""
		e.searchDir = 1
		if key == "?" {
			e.searchDir = -1
		}
		return
	}
	n := e.effectiveCount()
	// cw / cW act like ce / cE when the cursor is on a non-blank
	if op == "c" && (key == "w" || key == "W") {
		if m, ok := e.changeWordMotion(key == "W", maxInt(1, n)); ok {
			e.applyOperator(m)
			return
		}
	}
	m, ok := e.resolveMotion(key, n)
	e.finishMotion(m, ok)
}

// changeWordMotion implements the cw special case: change to the end of the
// word (without the following blanks), staying put when already at the end
// of a word.
func (e *Editor) changeWordMotion(big bool, n int) (motion, bool) {
	line := e.line()
	if e.col >= len(line) || classOf(line[e.col]) == 0 {
		return motion{}, false
	}
	p := pos{e.row, e.col}
	for i := 0; i < n; i++ {
		if i == 0 && e.atWordEnd(p, big) {
			continue
		}
		p = e.endWord(p, big)
	}
	return motion{to: p, kind: mInclusive}, true
}

func (e *Editor) atWordEnd(p pos, big bool) bool {
	line := e.lines[p.row]
	if p.col >= len(line) || classOf(line[p.col]) == 0 {
		return false
	}
	return p.col+1 >= len(line) || wordClass(line[p.col+1], big) != wordClass(line[p.col], big)
}

// applyOperator applies the pending operator from the cursor to a motion
// target, following vim's inclusive/exclusive/linewise rules.
func (e *Editor) applyOperator(m motion) {
	a, b := pos{e.row, e.col}, m.to
	if m.kind == mLinewise {
		e.applyLinewiseOp(a.row, b.row)
		return
	}
	if lessPos(b, a) {
		a, b = b, a
	}
	a.col = clampInt(a.col, 0, len(e.lines[a.row]))
	b.col = clampInt(b.col, 0, len(e.lines[b.row]))
	if m.kind == mInclusive {
		b.col = clampInt(b.col+1, 0, len(e.lines[b.row]))
	} else {
		// dw on the last word of a line stops at the end of the line
		// instead of eating the line break
		if m.wordFwd && b.row > a.row && len(e.lines[a.row]) > 0 && b.col <= firstNonBlank(e.lines[b.row]) {
			b = pos{b.row - 1, len(e.lines[b.row-1])}
		}
		// :help exclusive-linewise
		if b.row > a.row && b.col == 0 {
			if a.col <= firstNonBlank(e.lines[a.row]) {
				e.applyLinewiseOp(a.row, b.row-1)
				return
			}
			b = pos{b.row - 1, len(e.lines[b.row-1])}
		}
	}
	e.applyCharwiseOp(a, b)
}

// applyLinewiseOp applies the pending operator to whole lines from..to.
func (e *Editor) applyLinewiseOp(from, to int) {
	op := e.op
	e.resetPending()
	if from > to {
		from, to = to, from
	}
	from = clampInt(from, 0, len(e.lines)-1)
	to = clampInt(to, 0, len(e.lines)-1)
	switch op {
	case "y":
		e.setReg(e.linesText(from, to), true)
		e.row = from
		e.clamp()
	case "d":
		e.pushUndo()
		e.setReg(e.linesText(from, to), true)
		e.deleteLines(from, to)
		e.col = firstNonBlank(e.line())
	case "c":
		e.pushUndo()
		e.setReg(e.linesText(from, to), true)
		indent := append([]rune(nil), leadingBlanks(e.lines[from])...)
		all := from == 0 && to == len(e.lines)-1
		e.deleteLines(from, to)
		// the range is gone: leave one (indented) empty line in its place
		if all {
			e.lines = [][]rune{indent}
		} else {
			e.lines = append(e.lines[:from], append([][]rune{indent}, e.lines[from:]...)...)
		}
		e.row, e.col = from, len(indent)
		e.mode = Insert
	case ">", "<":
		e.pushUndo()
		e.shiftLines(from, to, op == ">", 1)
		e.row = from
		e.col = firstNonBlank(e.line())
	case "g~", "gu", "gU":
		e.pushUndo()
		for r := from; r <= to; r++ {
			transformRunes(e.lines[r], op)
		}
		e.row = from
		e.clamp()
	}
}

// applyCharwiseOp applies the pending operator to a charwise range (end
// exclusive).
func (e *Editor) applyCharwiseOp(a, b pos) {
	op := e.op
	e.resetPending()
	if lessPos(b, a) {
		a, b = b, a
	}
	switch op {
	case "y":
		e.setReg(e.extract(a, b), false)
		e.row, e.col = a.row, a.col
		e.clamp()
	case "d":
		e.pushUndo()
		e.setReg(e.extract(a, b), false)
		e.deleteRange(a, b)
	case "c":
		e.pushUndo()
		e.setReg(e.extract(a, b), false)
		e.mode = Insert // before deleteRange so the cursor may sit at end of line
		e.deleteRange(a, b)
	case ">", "<":
		to := b.row
		if b.col == 0 && b.row > a.row {
			to--
		}
		e.pushUndo()
		e.shiftLines(a.row, to, op == ">", 1)
		e.row = a.row
		e.col = firstNonBlank(e.line())
	case "g~", "gu", "gU":
		e.pushUndo()
		for r := a.row; r <= b.row; r++ {
			line := e.lines[r]
			from, to := 0, len(line)
			if r == a.row {
				from = clampInt(a.col, 0, len(line))
			}
			if r == b.row {
				to = clampInt(b.col, 0, len(line))
			}
			transformRunes(line[from:to], op)
		}
		e.row, e.col = a.row, a.col
		e.clamp()
	}
}

// ------------------------------------------------------------ primitives

func leadingBlanks(line []rune) []rune {
	i := 0
	for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
		i++
	}
	return line[:i]
}

func toggleCase(r rune) rune {
	if unicode.IsUpper(r) {
		return unicode.ToLower(r)
	}
	return unicode.ToUpper(r)
}

// transformRunes applies a case operator in place.
func transformRunes(rs []rune, op string) {
	for i, r := range rs {
		switch op {
		case "gu":
			rs[i] = unicode.ToLower(r)
		case "gU":
			rs[i] = unicode.ToUpper(r)
		default:
			rs[i] = toggleCase(r)
		}
	}
}

// shiftLines indents (right) or dedents lines from..to, times levels.
func (e *Editor) shiftLines(from, to int, right bool, times int) {
	if from > to {
		from, to = to, from
	}
	for r := from; r <= to && r < len(e.lines); r++ {
		line := e.lines[r]
		if right {
			if len(line) == 0 {
				continue // vim does not indent empty lines
			}
			e.lines[r] = append([]rune(strings.Repeat(" ", shiftWidth*times)), line...)
			continue
		}
		width := shiftWidth * times
		i := 0
		for i < len(line) && width > 0 {
			switch line[i] {
			case ' ':
				width--
			case '\t':
				width -= shiftWidth
			default:
				width = 0
				continue
			}
			i++
		}
		e.lines[r] = append([]rune(nil), line[i:]...)
	}
}

// joinLines joins the current line with the following ones `joins` times
// (J inserts a space, gJ does not).
func (e *Editor) joinLines(joins int, spaces bool) {
	for i := 0; i < joins && e.row < len(e.lines)-1; i++ {
		cur := e.line()
		next := e.lines[e.row+1]
		var trimmed []rune
		if spaces {
			trimmed = []rune(strings.TrimLeft(string(next), " \t"))
		} else {
			trimmed = next
		}
		joined := append([]rune(nil), cur...)
		e.col = len(cur)
		if spaces && len(cur) > 0 && len(trimmed) > 0 && trimmed[0] != ')' {
			last := cur[len(cur)-1]
			if last != ' ' && last != '\t' {
				joined = append(joined, ' ')
			}
		}
		if len(cur) > 0 && len(trimmed) == 0 && spaces {
			e.col = len(cur) - 1
		}
		joined = append(joined, trimmed...)
		e.lines[e.row] = joined
		e.lines = append(e.lines[:e.row+1], e.lines[e.row+2:]...)
	}
	e.clamp()
}

// replaceChars implements r{char}: n chars from the cursor in normal mode,
// every selected char in visual mode.
func (e *Editor) replaceChars(ch rune, n int) {
	e.resetPending()
	if e.mode == Visual || e.mode == VisualLine {
		a, b, _ := e.selRange()
		linewise := e.mode == VisualLine
		e.leaveVisual()
		e.pushUndo()
		for r := a.row; r <= b.row; r++ {
			line := e.lines[r]
			from, to := 0, len(line)
			if !linewise {
				if r == a.row {
					from = clampInt(a.col, 0, len(line))
				}
				if r == b.row {
					to = clampInt(b.col+1, 0, len(line))
				}
			}
			for c := from; c < to; c++ {
				line[c] = ch
			}
		}
		e.row, e.col = a.row, a.col
		if linewise {
			e.col = 0
		}
		e.clamp()
		return
	}
	line := e.line()
	if e.col+n > len(line) {
		return // not enough chars: vim beeps
	}
	e.pushUndo()
	for c := e.col; c < e.col+n; c++ {
		line[c] = ch
	}
	e.col += n - 1
	e.clamp()
}

// insertAt inserts (possibly multi-line) text at p and returns the position
// right after the inserted text.
func (e *Editor) insertAt(p pos, text string) pos {
	line := e.lines[p.row]
	c := clampInt(p.col, 0, len(line))
	head := append([]rune(nil), line[:c]...)
	tail := append([]rune(nil), line[c:]...)
	parts := strings.Split(text, "\n")
	if len(parts) == 1 {
		rs := []rune(parts[0])
		e.lines[p.row] = append(append(head, rs...), tail...)
		return pos{p.row, c + len(rs)}
	}
	block := make([][]rune, len(parts))
	for i, s := range parts {
		block[i] = []rune(s)
	}
	block[0] = append(head, block[0]...)
	lastIdx := len(block) - 1
	end := pos{p.row + lastIdx, len(block[lastIdx])}
	block[lastIdx] = append(block[lastIdx], tail...)
	e.lines = append(e.lines[:p.row], append(block, e.lines[p.row+1:]...)...)
	return end
}

// paste puts the register after/before the cursor n times. cursorAfter
// (gp/gP) leaves the cursor just after the pasted text.
func (e *Editor) paste(after, cursorAfter bool, n int) {
	if e.reg.text == "" {
		return
	}
	n = maxInt(1, n)
	if e.reg.linewise {
		var ins [][]rune
		for i := 0; i < n; i++ {
			for _, l := range strings.Split(e.reg.text, "\n") {
				ins = append(ins, []rune(l))
			}
		}
		at := e.row
		if after {
			at++
		}
		e.lines = append(e.lines[:at], append(ins, e.lines[at:]...)...)
		if cursorAfter {
			e.row = clampInt(at+len(ins), 0, len(e.lines)-1)
			e.col = 0
		} else {
			e.row = at
			e.col = firstNonBlank(e.line())
		}
		e.clamp()
		return
	}
	col := e.col
	if after && len(e.line()) > 0 {
		col++
	}
	end := e.insertAt(pos{e.row, col}, strings.Repeat(e.reg.text, n))
	if cursorAfter {
		e.row, e.col = end.row, end.col
		if e.col > e.maxColAt(e.row) { // past the end: vim goes to the next line start
			if e.row < len(e.lines)-1 && strings.Contains(e.reg.text, "\n") {
				e.row++
				e.col = 0
			}
		}
	} else {
		e.row, e.col = end.row, maxInt(0, end.col-1)
	}
	e.clamp()
}

// ----------------------------------------------------------- search prompt

// updateSearchPrompt handles keys while typing after / or ?.
func (e *Editor) updateSearchPrompt(msg tea.KeyMsg) {
	key := keyString(msg)
	switch key {
	case "esc":
		e.searching = false
		e.resetPending()
	case "enter":
		e.searching = false
		query := e.searchInput
		if query == "" {
			query = e.lastSearch
		}
		if query == "" {
			e.resetPending()
			return
		}
		e.lastSearch, e.lastSearchDir, e.lastWholeWord = query, e.searchDir, false
		m, ok := e.searchMotion(query, e.searchDir, false, maxInt(1, e.effectiveCount()))
		e.finishMotion(m, ok)
	case "backspace", "ctrl+h":
		if e.searchInput == "" {
			e.searching = false
			e.resetPending()
			return
		}
		rs := []rune(e.searchInput)
		e.searchInput = string(rs[:len(rs)-1])
	case "ctrl+u":
		e.searchInput = ""
	default:
		if msg.Type == tea.KeyRunes {
			e.searchInput += string(msg.Runes)
		} else if msg.Type == tea.KeySpace {
			e.searchInput += " "
		}
	}
}
