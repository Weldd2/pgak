package editor

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Insert-mode autocompletion: a small popup fed by candidate words supplied
// by the host (SQL keywords, table and column names). It opens automatically
// while typing (>= compAutoMinChars) or manually with ctrl+n.
const (
	compMaxVisible   = 6
	compAutoMinChars = 2
)

// SetCompletions sets the candidate words proposed in insert mode.
// Duplicates (case-insensitive) are removed, order is preserved.
func (e *Editor) SetCompletions(words []string) {
	seen := make(map[string]bool, len(words))
	out := make([]string, 0, len(words))
	for _, w := range words {
		k := strings.ToLower(w)
		if w == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, w)
	}
	e.completions = out
}

// CompletionActive reports whether the completion popup is open.
func (e *Editor) CompletionActive() bool { return e.compActive }

// compPrefix returns the start column and text of the identifier right
// before the cursor on the current line.
func (e *Editor) compPrefix() (int, string) {
	line := e.line()
	col := clampInt(e.col, 0, len(line))
	start := col
	for start > 0 && classOf(line[start-1]) == 1 {
		start--
	}
	return start, string(line[start:col])
}

// compFilter returns the candidates matching the current prefix
// (case-insensitive, prefix of at least minChars chars).
func (e *Editor) compFilter(minChars int) []string {
	_, prefix := e.compPrefix()
	if len([]rune(prefix)) < minChars {
		return nil
	}
	lp := strings.ToLower(prefix)
	var items []string
	for _, c := range e.completions {
		lc := strings.ToLower(c)
		if lc == lp {
			continue // already fully typed
		}
		if strings.HasPrefix(lc, lp) {
			items = append(items, c)
		}
	}
	return items
}

func (e *Editor) openCompletion(minChars int) {
	items := e.compFilter(minChars)
	if len(items) == 0 {
		e.closeCompletion()
		return
	}
	e.compItems = items
	e.compSel = 0
	e.compActive = true
}

// refreshCompletion re-filters after the buffer changed (closes the popup
// when nothing matches anymore).
func (e *Editor) refreshCompletion() {
	if !e.compActive {
		return
	}
	e.openCompletion(1)
}

func (e *Editor) closeCompletion() {
	e.compActive = false
	e.compItems = nil
	e.compSel = 0
}

// acceptCompletion replaces the typed prefix with the selected candidate.
// Uppercase candidates (SQL keywords) follow the case of the prefix.
func (e *Editor) acceptCompletion() {
	if !e.compActive || e.compSel >= len(e.compItems) {
		e.closeCompletion()
		return
	}
	cand := e.compItems[e.compSel]
	start, prefix := e.compPrefix()
	if cand == strings.ToUpper(cand) && prefix != "" && prefix == strings.ToLower(prefix) {
		cand = strings.ToLower(cand)
	}
	line := e.line()
	head := append([]rune(nil), line[:start]...)
	tail := append([]rune(nil), line[clampInt(e.col, 0, len(line)):]...)
	e.lines[e.row] = append(append(head, []rune(cand)...), tail...)
	e.col = start + len([]rune(cand))
	e.closeCompletion()
}

// handleCompletionKey processes keys while the popup is open; it returns
// true if the key was consumed.
func (e *Editor) handleCompletionKey(key string) bool {
	switch key {
	case "ctrl+n", "down":
		e.compSel = (e.compSel + 1) % len(e.compItems)
	case "ctrl+p", "up":
		e.compSel = (e.compSel - 1 + len(e.compItems)) % len(e.compItems)
	case "tab", "enter":
		e.acceptCompletion()
	case "esc":
		e.closeCompletion()
	default:
		return false
	}
	return true
}

// overlayCompletion draws the popup over the rendered editor view, anchored
// under (or above) the word being completed.
func (e *Editor) overlayCompletion(view string) string {
	rows := strings.Split(view, "\n")
	bannerOff := 0
	if e.banner != "" {
		bannerOff = 1
	}
	cursorScreen := e.row - e.scroll + bannerOff

	visible := minInt(len(e.compItems), compMaxVisible)
	placeAbove := false
	below := len(rows) - cursorScreen - 1
	if below < visible {
		above := cursorScreen - bannerOff
		if above >= below {
			placeAbove = true
			visible = minInt(visible, above)
		} else {
			visible = below
		}
	}
	if visible <= 0 {
		return view
	}

	// keep the selection visible in the window
	scroll := 0
	if e.compSel >= visible {
		scroll = e.compSel - visible + 1
	}

	// popup width and anchor column
	w := 0
	for _, it := range e.compItems {
		if l := len([]rune(it)); l > w {
			w = l
		}
	}
	numW := 4
	pw := clampInt(w+2, 4, maxInt(4, e.width-numW-2))

	start, _ := e.compPrefix()
	textW := maxInt(1, e.width-numW-1)
	hstart := 0
	if e.col >= textW {
		hstart = e.col - textW + 1
	}
	px := numW + 1 + start - hstart
	px = clampInt(px, numW+1, maxInt(numW+1, e.width-pw))

	for i := 0; i < visible; i++ {
		idx := scroll + i
		if idx >= len(e.compItems) {
			break
		}
		it := []rune(e.compItems[idx])
		if len(it) > pw-2 {
			it = append(it[:maxInt(0, pw-3)], '…')
		}
		item := " " + string(it) + strings.Repeat(" ", maxInt(0, pw-2-len(it))) + " "
		st := e.popupStyle
		if idx == e.compSel {
			st = e.popupSelStyle
		}
		target := cursorScreen + 1 + i
		if placeAbove {
			target = cursorScreen - visible + i
		}
		if target < 0 || target >= len(rows) {
			continue
		}
		base := ansi.Truncate(rows[target], px, "")
		if pad := px - ansi.StringWidth(base); pad > 0 {
			base += strings.Repeat(" ", pad)
		}
		rows[target] = base + st.Render(item)
	}
	return strings.Join(rows, "\n")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
