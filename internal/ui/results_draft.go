package ui

import (
	"github.com/mattn/go-runewidth"

	"pgak/internal/i18n"
)

// A draft is a row being composed in the grid before it is inserted: o opens
// an empty one, p a copy of the current row. Its cells are edited locally
// (i / c, x resets a cell to its default); ctrl+s asks the app to INSERT it
// and esc discards it. Only one draft exists at a time.
type draftRow struct {
	row      int          // index in Rows
	editable map[int]bool // columns of the target table
	set      map[int]bool // columns with an explicit value (others: DEFAULT)
}

// RowAction is a row-level request for the app, raised by a key.
type RowAction int

const (
	RowNone      RowAction = iota
	RowNew                 // o: open an empty draft row
	RowDuplicate           // p: open a draft row copied from the current one
	RowCommit              // ctrl+s: insert the draft row
)

// TakeRowAction returns the request raised by the last key (RowNone if none).
func (r *Results) TakeRowAction() RowAction {
	a := r.rowAction
	r.rowAction = RowNone
	return a
}

// HasDraft reports whether a draft row is being composed.
func (r *Results) HasDraft() bool { return r.draft != nil }

// CurRow / CurCol expose the cursor position.
func (r *Results) CurRow() int { return r.curRow }
func (r *Results) CurCol() int { return r.curCol }

// StartDraft opens a draft row under the current one and moves the cursor to
// it. editable lists the columns that can be filled, values the prefilled ones.
func (r *Results) StartDraft(editable map[int]bool, values map[int]string) {
	at := r.curRow + 1
	if at > len(r.Rows) {
		at = len(r.Rows)
	}
	row := make([]string, len(r.Cols))
	d := &draftRow{row: at, editable: editable, set: map[int]bool{}}
	r.Rows = append(r.Rows, nil)
	copy(r.Rows[at+1:], r.Rows[at:])
	r.Rows[at] = row
	r.draft = d
	for c, v := range values {
		if editable[c] {
			r.SetCell(at, c, v)
			d.set[c] = true
		}
	}
	r.curRow = at
	if !editable[r.curCol] {
		for c := range r.Cols {
			if editable[c] {
				r.curCol = c
				break
			}
		}
	}
	r.ensureVisible()
}

// DraftValues returns the explicit values of the draft row, by column.
func (r *Results) DraftValues() map[int]string {
	out := map[int]string{}
	if r.draft == nil {
		return out
	}
	for c := range r.draft.set {
		out[c] = r.cell(r.draft.row, c)
	}
	return out
}

// DiscardDraft removes the draft row.
func (r *Results) DiscardDraft() {
	if r.draft == nil {
		return
	}
	at := r.draft.row
	r.draft = nil
	r.CancelEdit()
	if at < len(r.Rows) {
		r.Rows = append(r.Rows[:at], r.Rows[at+1:]...)
	}
	if r.curRow >= at && r.curRow > 0 {
		r.curRow--
	}
	r.ensureVisible()
}

// CommitDraft turns the draft into a regular row holding the stored values
// (by column). Without a draft (discarded meanwhile) the row is appended.
func (r *Results) CommitDraft(stored map[int]string) {
	at := len(r.Rows)
	if r.draft != nil {
		at = r.draft.row
		r.draft = nil
	} else {
		r.Rows = append(r.Rows, make([]string, len(r.Cols)))
	}
	for c, v := range stored {
		r.SetCell(at, c, v)
	}
}

// onDraft reports whether the cursor is on the draft row.
func (r *Results) onDraft() bool { return r.draft != nil && r.curRow == r.draft.row }

// setDraftCell stores a locally edited value in the draft row.
func (r *Results) setDraftCell(col int, v string) {
	r.SetCell(r.draft.row, col, v)
	r.draft.set[col] = true
}

// resetDraftCell puts a draft cell back to "use the column default".
func (r *Results) resetDraftCell(col int) {
	if !r.draft.editable[col] {
		return
	}
	r.Rows[r.draft.row][col] = ""
	delete(r.draft.set, col)
}

// draftPlaceholder is shown in the draft cells without an explicit value.
func (r *Results) draftPlaceholder(col int) string {
	if !r.draft.editable[col] {
		return ""
	}
	s := i18n.T("results.draft.default")
	if w := r.widths[col]; runewidth.StringWidth(s) > w {
		s = runewidth.Truncate(s, w, "")
	}
	return s
}
