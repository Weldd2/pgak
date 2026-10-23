package ui

import (
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// Results is a vim-navigable table of query results.
type Results struct {
	Cols []string
	Rows [][]string

	curRow, curCol       int
	scrollRow, scrollCol int
	widths               []int

	pendingY bool
	pendingG bool
	count    int // count prefix (0 = none)

	width, height int
	focused       bool

	Status string // transient message (yank feedback), read by the app

	headStyle lipgloss.Style
	cellStyle lipgloss.Style
	curStyle  lipgloss.Style
	rowStyle  lipgloss.Style
	dimStyle  lipgloss.Style
}

func NewResults() *Results {
	return &Results{
		headStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("150")),
		curStyle:  lipgloss.NewStyle().Reverse(true).Bold(true),
		rowStyle:  lipgloss.NewStyle().Background(lipgloss.Color("236")),
		dimStyle:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
	}
}

const (
	maxColWidth = 40
	minColWidth = 3
)

func (r *Results) SetSize(w, h int)  { r.width, r.height = w, h }
func (r *Results) SetFocused(b bool) { r.focused = b }

func (r *Results) SetData(cols []string, rows [][]string) {
	r.Cols = cols
	r.Rows = rows
	r.curRow, r.curCol = 0, 0
	r.scrollRow, r.scrollCol = 0, 0
	r.pendingY, r.pendingG = false, false
	r.widths = make([]int, len(cols))
	for i, c := range cols {
		r.widths[i] = clampInt(runewidth.StringWidth(c), minColWidth, maxColWidth)
	}
	for _, row := range rows {
		for i, v := range row {
			if i >= len(r.widths) {
				continue
			}
			w := runewidth.StringWidth(v)
			if w > r.widths[i] {
				r.widths[i] = clampInt(w, minColWidth, maxColWidth)
			}
		}
	}
}

func (r *Results) Empty() bool { return len(r.Cols) == 0 }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// PendingKeys shows pending multi-key state for the status bar.
func (r *Results) PendingKeys() string {
	s := ""
	if r.count > 0 {
		s += strconv.Itoa(r.count)
	}
	if r.pendingY {
		s += "y"
	}
	if r.pendingG {
		s += "g"
	}
	return s
}

// takeCount consumes the count prefix (1 when unset).
func (r *Results) takeCount() int {
	n := r.count
	r.count = 0
	if n == 0 {
		return 1
	}
	return n
}

func (r *Results) Update(msg tea.KeyMsg) {
	if r.Empty() {
		return
	}
	key := msg.String()
	r.Status = ""

	if r.pendingY {
		r.pendingY = false
		r.count = 0
		r.yank(key)
		return
	}
	if r.pendingG {
		r.pendingG = false
		if key == "g" {
			if n := r.count; n > 0 {
				r.curRow = clampInt(n-1, 0, len(r.Rows)-1)
			} else {
				r.curRow = 0
			}
		}
		r.count = 0
		r.ensureVisible()
		return
	}

	// count prefix (bare 0 stays "first column")
	if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && !(key == "0" && r.count == 0) {
		r.count = r.count*10 + int(key[0]-'0')
		return
	}

	switch key {
	case "g": // keep the count for e.g. 5gg
		r.pendingG = true
		return
	case "y":
		r.pendingY = true
		r.count = 0
		return
	case "h", "left":
		r.curCol = clampInt(r.curCol-r.takeCount(), 0, len(r.Cols)-1)
	case "l", "right":
		r.curCol = clampInt(r.curCol+r.takeCount(), 0, len(r.Cols)-1)
	case "j", "down":
		r.curRow = clampInt(r.curRow+r.takeCount(), 0, len(r.Rows)-1)
	case "k", "up":
		r.curRow = clampInt(r.curRow-r.takeCount(), 0, len(r.Rows)-1)
	case "0", "^":
		r.curCol = 0
	case "$":
		r.curCol = len(r.Cols) - 1
	case "w":
		r.curCol = clampInt(r.curCol+r.takeCount(), 0, len(r.Cols)-1)
	case "b":
		r.curCol = clampInt(r.curCol-r.takeCount(), 0, len(r.Cols)-1)
	case "G":
		if n := r.count; n > 0 {
			r.curRow = clampInt(n-1, 0, len(r.Rows)-1)
		} else {
			r.curRow = len(r.Rows) - 1
		}
	case "ctrl+d":
		r.curRow = clampInt(r.curRow+r.pageSize()/2, 0, len(r.Rows)-1)
	case "ctrl+u":
		r.curRow = clampInt(r.curRow-r.pageSize()/2, 0, len(r.Rows)-1)
	case "ctrl+f", "pgdown":
		r.curRow = clampInt(r.curRow+r.pageSize(), 0, len(r.Rows)-1)
	case "ctrl+b", "pgup":
		r.curRow = clampInt(r.curRow-r.pageSize(), 0, len(r.Rows)-1)
	case "H": // top of screen
		r.curRow = clampInt(r.scrollRow, 0, len(r.Rows)-1)
	case "L": // bottom of screen
		r.curRow = clampInt(r.scrollRow+r.pageSize()-1, 0, len(r.Rows)-1)
	case "M": // middle of screen
		r.curRow = clampInt(r.scrollRow+r.pageSize()/2, 0, len(r.Rows)-1)
	}
	r.count = 0
	r.ensureVisible()
}

func (r *Results) pageSize() int {
	return maxInt(1, r.height-2)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (r *Results) cell(row, col int) string {
	if row < 0 || row >= len(r.Rows) {
		return ""
	}
	cells := r.Rows[row]
	if col < 0 || col >= len(cells) {
		return ""
	}
	return cells[col]
}

func (r *Results) yank(key string) {
	if len(r.Rows) == 0 {
		return
	}
	var text, what string
	switch key {
	case "y": // ligne
		text = strings.Join(r.Rows[r.curRow], "\t")
		what = "ligne"
	case "Y", "r": // ligne avec noms de colonnes
		text = strings.Join(r.Cols, "\t") + "\n" + strings.Join(r.Rows[r.curRow], "\t")
		what = "ligne + en-têtes"
	case "c": // cellule
		text = r.cell(r.curRow, r.curCol)
		what = "cellule"
	case "C": // cellule avec nom de colonne
		text = r.Cols[r.curCol] + ": " + r.cell(r.curRow, r.curCol)
		what = "cellule + colonne"
	case "o": // colonne entière
		vals := make([]string, 0, len(r.Rows))
		for i := range r.Rows {
			vals = append(vals, r.cell(i, r.curCol))
		}
		text = strings.Join(vals, "\n")
		what = "colonne"
	case "O": // colonne avec en-tête
		vals := []string{r.Cols[r.curCol]}
		for i := range r.Rows {
			vals = append(vals, r.cell(i, r.curCol))
		}
		text = strings.Join(vals, "\n")
		what = "colonne + en-tête"
	case "a": // tout (TSV avec header)
		var sb strings.Builder
		sb.WriteString(strings.Join(r.Cols, "\t"))
		for _, row := range r.Rows {
			sb.WriteByte('\n')
			sb.WriteString(strings.Join(row, "\t"))
		}
		text = sb.String()
		what = "tout le résultat"
	default:
		return
	}
	if err := clipboard.WriteAll(text); err != nil {
		r.Status = "erreur presse-papier: " + err.Error()
		return
	}
	r.Status = "copié: " + what
}

func (r *Results) ensureVisible() {
	ps := r.pageSize()
	if r.curRow < r.scrollRow {
		r.scrollRow = r.curRow
	}
	if r.curRow >= r.scrollRow+ps {
		r.scrollRow = r.curRow - ps + 1
	}
	// horizontal: make sure current column is visible
	if r.curCol < r.scrollCol {
		r.scrollCol = r.curCol
	}
	for r.visibleWidth(r.scrollCol, r.curCol) > r.width && r.scrollCol < r.curCol {
		r.scrollCol++
	}
}

// visibleWidth is the rendered width from column a to b inclusive.
func (r *Results) visibleWidth(a, b int) int {
	w := 0
	for i := a; i <= b && i < len(r.widths); i++ {
		w += r.widths[i] + 3 // " | "
	}
	return w
}

func (r *Results) View() string {
	if r.Empty() {
		return r.dimStyle.Render("Aucun résultat — exécutez une requête (ctrl+e)")
	}
	var sb strings.Builder

	// header
	var hd strings.Builder
	for c := r.scrollCol; c < len(r.Cols); c++ {
		cell := padCell(r.Cols[c], r.widths[c])
		if r.focused && c == r.curCol {
			hd.WriteString(r.headStyle.Underline(true).Render(cell))
		} else {
			hd.WriteString(r.headStyle.Render(cell))
		}
		hd.WriteString(r.dimStyle.Render(" │ "))
		if lipgloss.Width(hd.String()) > r.width {
			break
		}
	}
	sb.WriteString(truncLine(hd.String(), r.width))
	sb.WriteByte('\n')
	sb.WriteString(r.dimStyle.Render(strings.Repeat("─", maxInt(0, r.width))))

	ps := r.pageSize()
	for i := 0; i < ps; i++ {
		idx := r.scrollRow + i
		sb.WriteByte('\n')
		if idx >= len(r.Rows) {
			sb.WriteString(r.dimStyle.Render("~"))
			continue
		}
		var ln strings.Builder
		for c := r.scrollCol; c < len(r.Cols); c++ {
			cell := padCell(r.cell(idx, c), r.widths[c])
			switch {
			case r.focused && idx == r.curRow && c == r.curCol:
				ln.WriteString(r.curStyle.Render(cell))
			case r.focused && idx == r.curRow:
				ln.WriteString(r.rowStyle.Render(cell))
			default:
				ln.WriteString(cell)
			}
			ln.WriteString(r.dimStyle.Render(" │ "))
			if lipgloss.Width(ln.String()) > r.width {
				break
			}
		}
		sb.WriteString(truncLine(ln.String(), r.width))
	}
	return sb.String()
}

func padCell(s string, w int) string {
	s = strings.ReplaceAll(s, "\n", "␤")
	if runewidth.StringWidth(s) > w {
		s = runewidth.Truncate(s, w-1, "…")
	}
	return s + strings.Repeat(" ", maxInt(0, w-runewidth.StringWidth(s)))
}

func truncLine(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, "")
}
