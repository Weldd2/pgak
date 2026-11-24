package editor

// textObject resolves a text object key (after i or a) into a range.
// Charwise objects return an end-exclusive (a, b); linewise objects (ip,
// ap) return the first and last rows in a.row / b.row.
func (e *Editor) textObject(key string, around bool) (a, b pos, linewise, ok bool) {
	switch key {
	case "w", "W":
		a, b, ok = e.wordObject(key == "W", around)
	case `"`, "'", "`":
		a, b, ok = e.quoteObject(rune(key[0]), around)
	case "(", ")", "b":
		a, b, ok = e.pairObject('(', ')', around)
	case "{", "}", "B":
		a, b, ok = e.pairObject('{', '}', around)
	case "[", "]":
		a, b, ok = e.pairObject('[', ']', around)
	case "<", ">":
		a, b, ok = e.pairObject('<', '>', around)
	case "p":
		a, b, ok = e.paragraphObject(around)
		linewise = true
	case "s":
		a, b, ok = e.sentenceObject(around)
	}
	return a, b, linewise, ok
}

// wordObject implements iw/aw (and iW/aW for WORDs).
func (e *Editor) wordObject(big, around bool) (pos, pos, bool) {
	line := e.line()
	if len(line) == 0 {
		return pos{e.row, 0}, pos{e.row, 0}, true
	}
	col := clampInt(e.col, 0, len(line)-1)
	cl := wordClass(line[col], big)
	start, end := col, col+1
	for start > 0 && wordClass(line[start-1], big) == cl {
		start--
	}
	for end < len(line) && wordClass(line[end], big) == cl {
		end++
	}
	if around && cl != 0 {
		// aw: include trailing whitespace, else leading whitespace
		e0 := end
		for end < len(line) && classOf(line[end]) == 0 {
			end++
		}
		if end == e0 {
			for start > 0 && classOf(line[start-1]) == 0 {
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
				// a": include trailing blanks, else leading blanks
				end := b + 1
				for end < len(line) && classOf(line[end]) == 0 {
					end++
				}
				start := a
				if end == b+1 {
					for start > 0 && classOf(line[start-1]) == 0 {
						start--
					}
				}
				return pos{e.row, start}, pos{e.row, end}, true
			}
			return pos{e.row, a + 1}, pos{e.row, b}, true
		}
	}
	return pos{}, pos{}, false
}

// pairObject implements i(/a( and friends, searching the enclosing pair
// (multi-line, nesting-aware).
func (e *Editor) pairObject(open, close rune, around bool) (pos, pos, bool) {
	cur := pos{e.row, clampInt(e.col, 0, maxInt(0, len(e.line())))}

	// the bracket under the cursor is part of the pair we are looking for
	var openP pos
	switch e.charAt(cur) {
	case open:
		openP = cur
	case close:
		p, ok := e.unmatchedBracket(cur, open, close, true)
		if !ok {
			return pos{}, pos{}, false
		}
		openP = p
	default:
		p, ok := e.unmatchedBracket(cur, open, close, true)
		if !ok {
			return pos{}, pos{}, false
		}
		openP = p
	}
	closeP, ok := e.unmatchedBracket(openP, open, close, false)
	if !ok {
		return pos{}, pos{}, false
	}
	if around {
		return openP, pos{closeP.row, closeP.col + 1}, true
	}
	return pos{openP.row, openP.col + 1}, closeP, true
}

// paragraphObject implements ip/ap: the run of non-blank (or blank) lines
// around the cursor; ap adds the following blank lines (or the preceding
// ones when there are none after).
func (e *Editor) paragraphObject(around bool) (pos, pos, bool) {
	last := len(e.lines) - 1
	blank := isBlankLine(e.line())
	start, end := e.row, e.row
	for start > 0 && isBlankLine(e.lines[start-1]) == blank {
		start--
	}
	for end < last && isBlankLine(e.lines[end+1]) == blank {
		end++
	}
	if around {
		if end < last {
			// include the following run (blank lines after a paragraph, or
			// the paragraph after blank lines)
			next := !blank
			for end < last && isBlankLine(e.lines[end+1]) == next {
				end++
			}
		} else if !blank {
			for start > 0 && isBlankLine(e.lines[start-1]) {
				start--
			}
		}
	}
	return pos{start, 0}, pos{end, 0}, true
}

// sentenceObject implements is/as on the sentence under the cursor.
func (e *Editor) sentenceObject(around bool) (pos, pos, bool) {
	starts := e.sentenceStarts()
	if len(starts) == 0 {
		return pos{}, pos{}, false
	}
	cur := pos{e.row, e.col}
	i := 0
	for i+1 < len(starts) && !lessPos(cur, starts[i+1]) {
		i++
	}
	a := starts[i]
	last := len(e.lines) - 1
	end := pos{last, len(e.lines[last])}
	if i+1 < len(starts) {
		end = starts[i+1]
	}
	// b: just after the last non-blank char of the sentence
	b := end
	for {
		p, ok := e.prevPos(b)
		if !ok || !lessPos(a, p) && p != a {
			break
		}
		if ch := e.charAt(p); ch != ' ' && ch != '\t' && ch != '\n' {
			break
		}
		b = p
	}
	if !lessPos(a, b) {
		b = end
	}
	if around {
		line := e.lines[b.row]
		e0 := b.col
		for b.col < len(line) && classOf(line[b.col]) == 0 {
			b.col++
		}
		if b.col == e0 { // no trailing blanks on the line: take the leading ones
			aline := e.lines[a.row]
			for a.col > 0 && classOf(aline[a.col-1]) == 0 {
				a.col--
			}
		}
	}
	return a, b, true
}
