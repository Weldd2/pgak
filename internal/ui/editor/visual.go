package editor

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// leaveVisual returns to normal mode, remembering the selection for gv.
func (e *Editor) leaveVisual() {
	e.lastVis = visualSel{anchor: pos{e.vrow, e.vcol}, cursor: pos{e.row, e.col}, mode: e.mode, ok: true}
	e.mode = Normal
	e.resetPending()
	e.clamp()
}

func (e *Editor) updateVisual(msg tea.KeyMsg) {
	key, done := e.preKey(msg)
	if done {
		return
	}
	if m, ok := e.resolveMotion(key, e.count); ok {
		e.count = 0
		e.moveTo(m)
		return
	}
	n := maxInt(1, e.count)
	e.count = 0

	switch key {
	case "esc":
		e.leaveVisual()
	case "v":
		if e.mode == Visual {
			e.leaveVisual()
		} else {
			e.mode = Visual
		}
	case "V":
		if e.mode == VisualLine {
			e.leaveVisual()
		} else {
			e.mode = VisualLine
		}
	case "o", "O": // swap anchor and cursor
		e.vrow, e.row = e.row, e.vrow
		e.vcol, e.col = e.col, e.vcol
		e.want = e.col
	case "i", "a":
		e.objPend = rune(key[0])
	case "r":
		e.charPend = 'r'
	case "/", "?":
		e.searching = true
		e.searchInput = ""
		e.searchDir = 1
		if key == "?" {
			e.searchDir = -1
		}
	case "y":
		e.visualOp("y", false, 1)
	case "d", "x", "delete":
		e.visualOp("d", false, 1)
	case "c", "s":
		e.visualOp("c", false, 1)
	case "Y":
		e.visualOp("y", true, 1)
	case "D", "X":
		e.visualOp("d", true, 1)
	case "C", "S", "R":
		e.visualOp("c", true, 1)
	case ">", "<":
		e.visualOp(key, true, n)
	case "~":
		e.visualOp("g~", false, 1)
	case "u":
		e.visualOp("gu", false, 1)
	case "U":
		e.visualOp("gU", false, 1)
	case "g~", "gu", "gU":
		e.visualOp(key, false, 1)
	case "J", "gJ":
		a, b, _ := e.selRange()
		e.leaveVisual()
		e.pushUndo()
		e.row = a.row
		e.joinLines(maxInt(1, b.row-a.row), key == "J")
	case "p", "P":
		e.visualPaste()
	}
}

// visualSelectObject adjusts the selection to a text object (viw, va", vip…).
func (e *Editor) visualSelectObject(a, b pos, linewise bool) {
	if linewise {
		e.mode = VisualLine
		e.vrow, e.vcol = a.row, 0
		e.row, e.col = b.row, 0
		e.clamp()
		return
	}
	if !lessPos(a, b) {
		return
	}
	e.vrow, e.vcol = a.row, a.col
	// b is exclusive, the visual end is inclusive
	if b.col == 0 && b.row > a.row {
		e.row = b.row - 1
		e.col = maxInt(0, len(e.lines[e.row])-1)
	} else {
		e.row, e.col = b.row, maxInt(0, b.col-1)
	}
}

// visualOp applies an operator to the selection. forceLines treats a
// charwise selection as whole lines (Y, D, >, <…).
func (e *Editor) visualOp(op string, forceLines bool, times int) {
	linewise := forceLines || e.mode == VisualLine
	a, b, _ := e.selRange()
	e.leaveVisual()
	if op == ">" || op == "<" {
		e.pushUndo()
		e.shiftLines(a.row, b.row, op == ">", times)
		e.row = a.row
		e.col = firstNonBlank(e.line())
		return
	}
	e.op = op
	if linewise {
		e.applyLinewiseOp(a.row, b.row)
		return
	}
	b.col++ // visual selection is inclusive
	e.applyCharwiseOp(a, b)
}

// visualPaste replaces the selection with the register; the replaced text
// ends up in the register (vim behaviour).
func (e *Editor) visualPaste() {
	saved := e.reg
	linewise := e.mode == VisualLine
	a, b, _ := e.selRange()
	e.leaveVisual()
	if saved.text == "" {
		return
	}
	e.pushUndo()
	var deleted string
	if linewise {
		deleted = e.linesText(a.row, b.row)
		all := a.row == 0 && b.row == len(e.lines)-1
		e.deleteLines(a.row, b.row)
		at := a.row
		var ins [][]rune
		for _, l := range strings.Split(saved.text, "\n") {
			ins = append(ins, []rune(l))
		}
		if all { // whole buffer replaced: drop the placeholder empty line
			e.lines = ins
		} else {
			e.lines = append(e.lines[:at], append(ins, e.lines[at:]...)...)
		}
		e.row, e.col = at, firstNonBlank(e.lines[at])
	} else {
		b.col++
		deleted = e.extract(a, b)
		e.deleteRange(a, b)
		text := saved.text
		if saved.linewise {
			// putting lines inside a line: split it around them
			text = "\n" + text + "\n"
		}
		end := e.insertAt(a, text)
		e.row, e.col = end.row, maxInt(0, end.col-1)
	}
	e.reg = register{deleted, linewise}
	e.yankToClipboard(deleted)
	e.clamp()
}
