package editor

import (
	"strings"
	"unicode"
)

// motionKind follows vim: an exclusive motion does not include the target
// char, an inclusive one does, a linewise one operates on whole lines.
type motionKind int

const (
	mExclusive motionKind = iota
	mInclusive
	mLinewise
)

// motion is the resolved target of a movement key.
type motion struct {
	to       pos
	kind     motionKind
	jump     bool // sets the '' mark
	keepWant bool // j/k: do not update the desired column
	wantEnd  bool // $: stick to the end of line on j/k
	wordFwd  bool // w/W: special operator behaviour at end of line
}

// resolveMotion maps a (possibly prefixed) key to a motion. count is the
// effective count (0 = none). ok is false if key is not a motion.
func (e *Editor) resolveMotion(key string, count int) (motion, bool) {
	n := maxInt(1, count)
	cur := pos{e.row, e.col}
	last := len(e.lines) - 1

	switch key {
	// ---- left / right
	case "h", "left", "backspace", "ctrl+h":
		return motion{to: pos{e.row, e.col - n}}, true
	case "l", "right", " ":
		return motion{to: pos{e.row, e.col + n}}, true
	case "0", "home":
		return motion{to: pos{e.row, 0}}, true
	case "^", "g^", "g0":
		return motion{to: pos{e.row, firstNonBlank(e.line())}}, true
	case "$", "end", "g$":
		r := clampInt(e.row+n-1, 0, last)
		return motion{to: pos{r, maxInt(0, len(e.lines[r])-1)}, kind: mInclusive, wantEnd: true}, true
	case "g_":
		r := clampInt(e.row+n-1, 0, last)
		return motion{to: pos{r, lastNonBlank(e.lines[r])}, kind: mInclusive}, true
	case "gm", "gM":
		return motion{to: pos{e.row, len(e.line()) / 2}}, true
	case "|":
		return motion{to: pos{e.row, n - 1}}, true

	// ---- up / down
	case "j", "down":
		r := clampInt(e.row+n, 0, last)
		return motion{to: pos{r, e.wantCol(r)}, kind: mLinewise, keepWant: true}, true
	case "k", "up":
		r := clampInt(e.row-n, 0, last)
		return motion{to: pos{r, e.wantCol(r)}, kind: mLinewise, keepWant: true}, true
	case "+", "enter", "ctrl+m":
		r := clampInt(e.row+n, 0, last)
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise}, true
	case "-":
		r := clampInt(e.row-n, 0, last)
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise}, true
	case "_":
		r := clampInt(e.row+n-1, 0, last)
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise}, true
	case "gg":
		r := 0
		if count > 0 {
			r = clampInt(count-1, 0, last)
		}
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise, jump: true}, true
	case "G":
		r := last
		if count > 0 {
			r = clampInt(count-1, 0, last)
		}
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise, jump: true}, true

	// ---- screen-relative
	case "H":
		r := clampInt(e.scroll+n-1, 0, last)
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise, jump: true}, true
	case "L":
		bottom := clampInt(e.scroll+e.contentHeight()-1, 0, last)
		r := clampInt(bottom-(n-1), 0, last)
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise, jump: true}, true
	case "M":
		bottom := clampInt(e.scroll+e.contentHeight()-1, 0, last)
		r := (e.scroll + bottom) / 2
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise, jump: true}, true
	case "ctrl+d", "ctrl+u":
		half := maxInt(1, e.contentHeight()/2)
		if count > 0 {
			half = count
		}
		if key == "ctrl+u" {
			half = -half
		}
		r := clampInt(e.row+half, 0, last)
		e.scroll = clampInt(e.scroll+half, 0, last)
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise}, true
	case "ctrl+f", "pgdown", "ctrl+b", "pgup":
		page := maxInt(1, e.contentHeight()-2) * n
		if key == "ctrl+b" || key == "pgup" {
			page = -page
		}
		r := clampInt(e.row+page, 0, last)
		e.scroll = clampInt(e.scroll+page, 0, last)
		return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise}, true

	// ---- words
	case "w", "W":
		p := cur
		for i := 0; i < n; i++ {
			p = e.nextWord(p, key == "W")
		}
		return motion{to: p, wordFwd: true}, true
	case "b", "B":
		p := cur
		for i := 0; i < n; i++ {
			p = e.prevWord(p, key == "B")
		}
		return motion{to: p}, true
	case "e", "E":
		p := cur
		for i := 0; i < n; i++ {
			p = e.endWord(p, key == "E")
		}
		return motion{to: p, kind: mInclusive}, true
	case "ge", "gE":
		p := cur
		for i := 0; i < n; i++ {
			p = e.prevEndWord(p, key == "gE")
		}
		return motion{to: p, kind: mInclusive}, true

	// ---- repeat find
	case ";", ",":
		if e.lastFind[0] == 0 {
			return motion{}, false
		}
		cmd := e.lastFind[0]
		if key == "," {
			cmd = reverseFind(cmd)
		}
		return e.findMotion(cmd, e.lastFind[1], n, true)

	// ---- paragraphs / sentences
	case "}":
		p := cur
		for i := 0; i < n; i++ {
			p = e.nextParagraph(p)
		}
		return motion{to: p, jump: true}, true
	case "{":
		p := cur
		for i := 0; i < n; i++ {
			p = e.prevParagraph(p)
		}
		return motion{to: p, jump: true}, true
	case ")":
		p := cur
		for i := 0; i < n; i++ {
			p = e.nextSentence(p)
		}
		return motion{to: p, jump: true}, true
	case "(":
		p := cur
		for i := 0; i < n; i++ {
			p = e.prevSentence(p)
		}
		return motion{to: p, jump: true}, true

	// ---- brackets
	case "%":
		if count > 0 { // N%: go to N percent of the file
			r := clampInt((count*len(e.lines)+99)/100-1, 0, last)
			return motion{to: pos{r, firstNonBlank(e.lines[r])}, kind: mLinewise, jump: true}, true
		}
		m, ok := e.matchBracket()
		if !ok {
			return motion{}, false
		}
		return motion{to: m, kind: mInclusive, jump: true}, true
	case "[(", "[{", "])", "]}":
		open, close := '(', ')'
		if key[1] == '{' || key[1] == '}' {
			open, close = '{', '}'
		}
		p := cur
		for i := 0; i < n; i++ {
			q, ok := e.unmatchedBracket(p, open, close, key[0] == '[')
			if !ok {
				if i == 0 {
					return motion{}, false
				}
				break
			}
			p = q
		}
		return motion{to: p, jump: true}, true

	// ---- search
	case "n", "N":
		if e.lastSearch == "" {
			return motion{}, false
		}
		dir := e.lastSearchDir
		if key == "N" {
			dir = -dir
		}
		return e.searchMotion(e.lastSearch, dir, e.lastWholeWord, n)
	case "*", "#", "g*", "g#":
		word, ok := e.wordUnderCursor()
		if !ok {
			return motion{}, false
		}
		dir := 1
		if strings.HasSuffix(key, "#") {
			dir = -1
		}
		whole := !strings.HasPrefix(key, "g")
		e.lastSearch, e.lastSearchDir, e.lastWholeWord = word, dir, whole
		return e.searchMotion(word, dir, whole, n)
	}
	return motion{}, false
}

// wantCol is the column j/k should land on in row (desired column, clamped).
func (e *Editor) wantCol(row int) int {
	return clampInt(e.want, 0, e.maxColAt(row))
}

// moveTo moves the cursor according to a motion (no operator).
func (e *Editor) moveTo(m motion) {
	if m.jump {
		e.setJump()
	}
	e.row = clampInt(m.to.row, 0, len(e.lines)-1)
	e.col = m.to.col
	e.clamp()
	switch {
	case m.keepWant:
	case m.wantEnd:
		e.want = 1 << 30
	default:
		e.want = e.col
	}
}

// setJump records the position before a jump (” and “ go back to it).
func (e *Editor) setJump() {
	e.jumpMark = pos{e.row, e.col}
	e.hasJump = true
}

// ------------------------------------------------------------- word logic

// classOf: 0 blank, 1 keyword char, 2 other punctuation.
func classOf(r rune) int {
	switch {
	case r == ' ' || r == '\t':
		return 0
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
		return 1
	default:
		return 2
	}
}

// wordClass is classOf, or 1 for every non-blank with WORD motions.
func wordClass(r rune, big bool) int {
	c := classOf(r)
	if big && c == 2 {
		return 1
	}
	return c
}

// nextWord returns the start of the next word (w / W). An empty line is a
// word.
func (e *Editor) nextWord(p pos, big bool) pos {
	line := e.lines[p.row]
	col := p.col
	if col < len(line) {
		cl := wordClass(line[col], big)
		for col < len(line) && wordClass(line[col], big) == cl {
			col++
		}
		for col < len(line) && classOf(line[col]) == 0 {
			col++
		}
		if col < len(line) {
			return pos{p.row, col}
		}
	}
	for r := p.row + 1; r < len(e.lines); r++ {
		l := e.lines[r]
		c := 0
		for c < len(l) && classOf(l[c]) == 0 {
			c++
		}
		if c < len(l) || len(l) == 0 {
			return pos{r, c}
		}
	}
	last := len(e.lines) - 1
	return pos{last, maxInt(0, len(e.lines[last]))}
}

// endWord returns the end of the current/next word (e / E).
func (e *Editor) endWord(p pos, big bool) pos {
	row, col := p.row, p.col
	for {
		line := e.lines[row]
		col++
		for col < len(line) && classOf(line[col]) == 0 {
			col++
		}
		if col < len(line) {
			cl := wordClass(line[col], big)
			for col+1 < len(line) && wordClass(line[col+1], big) == cl {
				col++
			}
			return pos{row, col}
		}
		if row+1 >= len(e.lines) {
			return pos{row, maxInt(0, len(line)-1)}
		}
		row++
		col = -1
	}
}

// prevWord returns the start of the previous word (b / B).
func (e *Editor) prevWord(p pos, big bool) pos {
	row, col := p.row, p.col
	for {
		line := e.lines[row]
		col--
		for col >= 0 && classOf(line[col]) == 0 {
			col--
		}
		if col >= 0 {
			cl := wordClass(line[col], big)
			for col-1 >= 0 && wordClass(line[col-1], big) == cl {
				col--
			}
			return pos{row, col}
		}
		if row == 0 {
			return pos{0, 0}
		}
		row--
		col = len(e.lines[row])
		if col == 0 { // an empty line is a word
			return pos{row, 0}
		}
	}
}

// prevEndWord returns the end of the previous word (ge / gE).
func (e *Editor) prevEndWord(p pos, big bool) pos {
	row, col := p.row, p.col
	line := e.lines[row]
	// leave the current word
	if col < len(line) {
		cl := wordClass(line[col], big)
		if cl != 0 {
			for col >= 0 && wordClass(line[col], big) == cl {
				col--
			}
		}
	} else {
		col = len(line) - 1
	}
	for {
		line = e.lines[row]
		for col >= 0 && classOf(line[col]) == 0 {
			col--
		}
		if col >= 0 {
			return pos{row, col}
		}
		if row == 0 {
			return pos{0, 0}
		}
		row--
		col = len(e.lines[row]) - 1
		if col < 0 { // empty line counts as a word
			return pos{row, 0}
		}
	}
}

// wordUnderCursor returns the keyword under (or after) the cursor on the
// current line, for * and #.
func (e *Editor) wordUnderCursor() (string, bool) {
	line := e.line()
	c := e.col
	for c < len(line) && classOf(line[c]) != 1 {
		c++
	}
	if c >= len(line) {
		return "", false
	}
	s := c
	for s > 0 && classOf(line[s-1]) == 1 {
		s--
	}
	t := c
	for t < len(line) && classOf(line[t]) == 1 {
		t++
	}
	return string(line[s:t]), true
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

// findMotion resolves f/F/t/T (n times) into a motion. f and t are
// inclusive, F and T exclusive. repeat is set for ; and , so that a
// repeated t/T does not stay stuck in front of the same char.
func (e *Editor) findMotion(cmd, ch rune, n int, repeat bool) (motion, bool) {
	col := e.col
	line := e.line()
	for i := 0; i < n; i++ {
		var ok bool
		start := col
		// ";" after t must skip the char right in front of us
		if i > 0 || repeat {
			if cmd == 't' {
				start = col + 1
			} else if cmd == 'T' {
				start = col - 1
			}
		}
		col, ok = findOnLine(line, start, cmd, ch)
		if !ok {
			return motion{}, false
		}
	}
	kind := mExclusive
	if cmd == 'f' || cmd == 't' {
		kind = mInclusive
	}
	return motion{to: pos{e.row, col}, kind: kind}, true
}

// ------------------------------------------------------------- paragraphs

// nextParagraph moves to the next blank line after the current paragraph
// (}), or to the end of the buffer.
func (e *Editor) nextParagraph(p pos) pos {
	last := len(e.lines) - 1
	r := p.row
	for r < last && len(e.lines[r]) == 0 {
		r++
	}
	for r < last && len(e.lines[r]) != 0 {
		r++
	}
	if len(e.lines[r]) == 0 {
		return pos{r, 0}
	}
	return pos{r, maxInt(0, len(e.lines[r])-1)}
}

// prevParagraph moves to the previous blank line ({), or to the start.
func (e *Editor) prevParagraph(p pos) pos {
	r := p.row
	for r > 0 && len(e.lines[r]) == 0 {
		r--
	}
	for r > 0 && len(e.lines[r]) != 0 {
		r--
	}
	return pos{r, 0}
}

// -------------------------------------------------------------- sentences

// sentenceStarts lists the positions where sentences begin: the first
// non-blank after ". ", "! " or "? " (closing quotes/brackets allowed), the
// first non-blank after an empty line, and every empty line.
func (e *Editor) sentenceStarts() []pos {
	var starts []pos
	needStart := true
	sawTerm := false
	for r, line := range e.lines {
		if len(line) == 0 {
			starts = append(starts, pos{r, 0})
			needStart, sawTerm = true, false
			continue
		}
		for c, ch := range line {
			if classOf(ch) == 0 {
				if sawTerm {
					needStart = true
				}
				continue
			}
			if needStart {
				starts = append(starts, pos{r, c})
				needStart = false
			}
			switch {
			case ch == '.' || ch == '!' || ch == '?':
				sawTerm = true
			case sawTerm && strings.ContainsRune(`)]"'`, ch):
			default:
				sawTerm = false
			}
		}
		if sawTerm { // end of line counts as whitespace
			needStart = true
		}
	}
	return starts
}

func (e *Editor) nextSentence(p pos) pos {
	for _, s := range e.sentenceStarts() {
		if lessPos(p, s) {
			return s
		}
	}
	last := len(e.lines) - 1
	return pos{last, maxInt(0, len(e.lines[last])-1)}
}

func (e *Editor) prevSentence(p pos) pos {
	prev := pos{0, 0}
	for _, s := range e.sentenceStarts() {
		if !lessPos(s, p) {
			break
		}
		prev = s
	}
	return prev
}

// --------------------------------------------------------------- brackets

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
		return e.unmatchedBracket(start, line[c], rune(closes[i]), false)
	}
	i := strings.IndexRune(closes, line[c])
	return e.unmatchedBracket(start, rune(opens[i]), line[c], true)
}

// unmatchedBracket scans from p (exclusive) for the enclosing unmatched
// open (backward=true) or close bracket.
func (e *Editor) unmatchedBracket(p pos, open, close rune, backward bool) (pos, bool) {
	depth := 0
	if backward {
		q, ok := e.prevPos(p)
		for ok {
			switch e.charAt(q) {
			case open:
				if depth == 0 {
					return q, true
				}
				depth--
			case close:
				depth++
			}
			q, ok = e.prevPos(q)
		}
		return pos{}, false
	}
	q, ok := e.nextPos(p)
	for ok {
		switch e.charAt(q) {
		case close:
			if depth == 0 {
				return q, true
			}
			depth--
		case open:
			depth++
		}
		q, ok = e.nextPos(q)
	}
	return pos{}, false
}

// ----------------------------------------------------------------- search

// foldCase reports whether a search is case-insensitive (smartcase: no
// uppercase letter in the pattern).
func foldCase(q string) bool {
	return strings.ToLower(q) == q
}

func lowerRunes(rs []rune) []rune {
	out := make([]rune, len(rs))
	for i, r := range rs {
		out[i] = unicode.ToLower(r)
	}
	return out
}

// matchAt reports whether needle occurs in line at col (whole-word aware).
func matchAt(line, needle []rune, col int, whole bool) bool {
	if col < 0 || col+len(needle) > len(line) {
		return false
	}
	for i, r := range needle {
		if line[col+i] != r {
			return false
		}
	}
	if whole {
		if col > 0 && classOf(line[col-1]) == 1 {
			return false
		}
		if end := col + len(needle); end < len(line) && classOf(line[end]) == 1 {
			return false
		}
	}
	return true
}

// searchFrom finds the next (dir>0) or previous (dir<0) match strictly
// after/before p, wrapping around the buffer.
func (e *Editor) searchFrom(p pos, query string, dir int, whole bool) (pos, bool) {
	needle := []rune(query)
	fold := foldCase(query)
	if fold {
		needle = lowerRunes(needle)
	}
	if len(needle) == 0 {
		return pos{}, false
	}
	nrows := len(e.lines)
	for i := 0; i <= nrows; i++ { // nrows+1 passes: the start row is visited twice (wrap)
		r := ((p.row+dir*i)%nrows + nrows) % nrows
		line := e.lines[r]
		if fold {
			line = lowerRunes(line)
		}
		if dir > 0 {
			from := 0
			if i == 0 {
				from = p.col + 1
			}
			to := len(line) - len(needle)
			if i == nrows {
				to = p.col // wrapped back to the start row: up to the cursor
			}
			for c := from; c <= to; c++ {
				if matchAt(line, needle, c, whole) {
					return pos{r, c}, true
				}
			}
		} else {
			from := len(line) - len(needle)
			if i == 0 {
				from = p.col - 1
			}
			to := 0
			if i == nrows {
				to = p.col
			}
			for c := from; c >= to; c-- {
				if matchAt(line, needle, c, whole) {
					return pos{r, c}, true
				}
			}
		}
	}
	return pos{}, false
}

// searchMotion resolves the n-th match of query from the cursor.
func (e *Editor) searchMotion(query string, dir int, whole bool, n int) (motion, bool) {
	p := pos{e.row, e.col}
	for i := 0; i < n; i++ {
		q, ok := e.searchFrom(p, query, dir, whole)
		if !ok {
			return motion{}, false
		}
		p = q
	}
	return motion{to: p, jump: true}, true
}

// ------------------------------------------------------------------ marks

// markMotion resolves '{mark} (linewise) and `{mark} (exclusive).
func (e *Editor) markMotion(cmd, ch rune) (motion, bool) {
	var p pos
	switch {
	case ch == '\'' || ch == '`':
		if !e.hasJump {
			return motion{}, false
		}
		p = e.jumpMark
	case ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z':
		q, ok := e.marks[ch]
		if !ok {
			return motion{}, false
		}
		p = q
	default:
		return motion{}, false
	}
	p = e.clampPos(p)
	if cmd == '\'' {
		return motion{to: pos{p.row, firstNonBlank(e.lines[p.row])}, kind: mLinewise, jump: true}, true
	}
	return motion{to: p, jump: true}, true
}
