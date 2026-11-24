package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"

	"pgak/internal/i18n"
)

// CellEdit is a committed cell edition, to be applied by the app.
type CellEdit struct {
	Row, Col int
	Value    string
}

// Editing reports whether the cell editor is open.
func (r *Results) Editing() bool { return r.editing }

// TakeEdit returns the edition committed by the last key (nil if none).
func (r *Results) TakeEdit() *CellEdit {
	e := r.edit
	r.edit = nil
	return e
}

// CancelEdit closes the cell editor without committing.
func (r *Results) CancelEdit() {
	r.editing = false
	r.editBuf = nil
	r.editCur = 0
}

// startEdit opens the cell editor on the current cell. keep=false starts
// from an empty value (vim's c), keep=true from the current one (i).
func (r *Results) startEdit(keep bool) {
	if len(r.Rows) == 0 {
		return
	}
	if r.onDraft() {
		if !r.draft.editable[r.curCol] {
			r.Status = i18n.T("status.error.row.othercol")
			return
		}
	} else if r.CanEdit != nil {
		if err := r.CanEdit(r.curCol); err != nil {
			r.Status = err.Error()
			return
		}
	}
	r.editing = true
	r.editBuf = nil
	if keep {
		r.editBuf = []rune(r.cell(r.curRow, r.curCol))
	}
	r.editCur = len(r.editBuf)
}

// updateEdit handles keys while the cell editor is open.
func (r *Results) updateEdit(msg tea.KeyMsg) {
	switch msg.String() {
	case "enter":
		if r.onDraft() { // local until the row is inserted
			r.setDraftCell(r.curCol, string(r.editBuf))
		} else {
			r.edit = &CellEdit{Row: r.curRow, Col: r.curCol, Value: string(r.editBuf)}
		}
		r.CancelEdit()
	case "esc":
		r.CancelEdit()
	case "left", "ctrl+b":
		if r.editCur > 0 {
			r.editCur--
		}
	case "right", "ctrl+f":
		if r.editCur < len(r.editBuf) {
			r.editCur++
		}
	case "home", "ctrl+a":
		r.editCur = 0
	case "end", "ctrl+e":
		r.editCur = len(r.editBuf)
	case "backspace", "ctrl+h":
		if r.editCur > 0 {
			r.editBuf = append(r.editBuf[:r.editCur-1], r.editBuf[r.editCur:]...)
			r.editCur--
		}
	case "delete", "ctrl+d":
		if r.editCur < len(r.editBuf) {
			r.editBuf = append(r.editBuf[:r.editCur], r.editBuf[r.editCur+1:]...)
		}
	case "ctrl+u": // delete to the start
		r.editBuf = append([]rune{}, r.editBuf[r.editCur:]...)
		r.editCur = 0
	case "ctrl+k": // delete to the end
		r.editBuf = r.editBuf[:r.editCur]
	case "ctrl+w": // delete the word before the cursor
		i := r.editCur
		for i > 0 && r.editBuf[i-1] == ' ' {
			i--
		}
		for i > 0 && r.editBuf[i-1] != ' ' {
			i--
		}
		r.editBuf = append(r.editBuf[:i], r.editBuf[r.editCur:]...)
		r.editCur = i
	case " ", "space":
		r.insertRunes([]rune{' '})
	default:
		if msg.Type == tea.KeyRunes {
			r.insertRunes(msg.Runes)
		}
	}
}

func (r *Results) insertRunes(rs []rune) {
	buf := make([]rune, 0, len(r.editBuf)+len(rs))
	buf = append(buf, r.editBuf[:r.editCur]...)
	buf = append(buf, rs...)
	buf = append(buf, r.editBuf[r.editCur:]...)
	r.editBuf = buf
	r.editCur += len(rs)
}

// editView renders the cell editor in a field of width w: the text scrolled
// so the cursor is visible, the cursor drawn in reverse video.
func (r *Results) editView(w int) string {
	if w < 2 {
		w = 2
	}
	buf := r.editBuf
	cur := r.editCur
	// window [start, end) of runes to show, cursor needs one cell at the end
	start := 0
	for widthOf(buf[start:cur])+1 > w {
		start++
	}
	end := cur
	for end < len(buf) && widthOf(buf[start:end+1])+1 <= w {
		end++
	}
	before := string(buf[start:cur])
	under := " "
	after := ""
	if cur < len(buf) {
		under = string(buf[cur])
		if cur+1 <= end {
			after = string(buf[cur+1 : end])
		}
	}
	text := before + r.editCurStyle.Render(under) + after
	pad := w - widthOf(buf[start:cur]) - runewidth.StringWidth(under) - runewidth.StringWidth(after)
	if pad < 0 {
		pad = 0
	}
	return r.editStyle.Render(text + strings.Repeat(" ", pad))
}

func widthOf(rs []rune) int {
	return runewidth.StringWidth(strings.ReplaceAll(string(rs), "\n", "␤"))
}

// SetCell replaces the value of a cell (after a committed edition) and widens
// the column if needed.
func (r *Results) SetCell(row, col int, v string) {
	if row < 0 || row >= len(r.Rows) || col < 0 || col >= len(r.Rows[row]) {
		return
	}
	r.Rows[row][col] = v
	if col < len(r.widths) {
		if w := runewidth.StringWidth(v); w > r.widths[col] {
			r.widths[col] = clampInt(w, minColWidth, maxColWidth)
		}
	}
}

// rowHasKey reports whether row has the given values in the key columns.
func (r *Results) rowHasKey(row int, keyCols []int, keyVals []string) bool {
	for i, c := range keyCols {
		if r.cell(row, c) != keyVals[i] {
			return false
		}
	}
	return true
}
