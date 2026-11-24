package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"pgak/internal/db"
)

// resultTab is one result set (or error) of a multi-statement run.
type resultTab struct {
	sql   string   // the statement that produced it
	label string   // short title shown in the tab bar
	res   *Results // the navigable table (or an error view)
	info  string   // "42 rows · 12ms", shown in the zone title
	err   bool     // the statement failed

	result   *db.Result // raw result (column origins, for cell editing)
	connName string     // connection / database the result comes from:
	database string     // cell edits are refused after switching

	insert db.InsertTarget // table the draft row (o / p) goes to
}

const tabLabelWidth = 18

// tabLabel squashes a statement onto one line and truncates it.
func tabLabel(sql string) string {
	s := strings.Join(strings.Fields(sql), " ")
	if runewidth.StringWidth(s) > tabLabelWidth {
		s = runewidth.Truncate(s, tabLabelWidth, "…")
	}
	return s
}

// showTabBar reports whether the tab bar occupies a line of the results zone.
func (a *App) showTabBar() bool { return len(a.tabs) > 1 }

// selectTab makes tab i the visible one (clamped).
func (a *App) selectTab(i int) {
	if len(a.tabs) == 0 {
		return
	}
	a.curTab = clampInt(i, 0, len(a.tabs)-1)
	a.res = a.tabs[a.curTab].res
	a.resInfo = a.tabs[a.curTab].info
	a.layout()
}

// nextTab / prevTab move between result tabs, wrapping around.
func (a *App) nextTab() {
	if len(a.tabs) > 1 {
		a.selectTab((a.curTab + 1) % len(a.tabs))
	}
}

func (a *App) prevTab() {
	if len(a.tabs) > 1 {
		a.selectTab((a.curTab + len(a.tabs) - 1) % len(a.tabs))
	}
}

var (
	tabStyleCur    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("235")).Background(colAccent).Padding(0, 1)
	tabStyleDim    = lipgloss.NewStyle().Foreground(colDim).Padding(0, 1)
	tabStyleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Padding(0, 1)
	tabStyleCurErr = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("231")).Background(lipgloss.Color("131")).Padding(0, 1)
)

// tabBar renders the row of tabs, scrolled so the current tab is visible.
func (a *App) tabBar(width int) string {
	segs := make([]string, len(a.tabs))
	for i, t := range a.tabs {
		txt := strconv.Itoa(i+1) + ":" + t.label
		switch {
		case i == a.curTab && t.err:
			segs[i] = tabStyleCurErr.Render(txt)
		case i == a.curTab:
			segs[i] = tabStyleCur.Render(txt)
		case t.err:
			segs[i] = tabStyleErr.Render(txt)
		default:
			segs[i] = tabStyleDim.Render(txt)
		}
	}
	// scroll: drop leading tabs until the current one fits
	start := 0
	for start < a.curTab && lipgloss.Width(strings.Join(segs[start:a.curTab+1], "")) > width {
		start++
	}
	line := strings.Join(segs[start:], "")
	if start > 0 {
		line = dimText.Render("‹") + line
	}
	return truncLine(line, width)
}
