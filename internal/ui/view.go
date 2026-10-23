package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"pgak/internal/i18n"
	"pgak/internal/ui/editor"
)

var (
	colBlue   = lipgloss.Color("68")
	colRed    = lipgloss.Color("131")
	colGreen  = lipgloss.Color("71")
	colDim    = lipgloss.Color("240")
	colText   = lipgloss.Color("252")
	colAccent = lipgloss.Color("221")

	dimBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colDim)

	statusOK  = lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
	statusBad = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	dimText   = lipgloss.NewStyle().Foreground(colDim)
	accText   = lipgloss.NewStyle().Foreground(colAccent)

	modeStyleNormal = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("68")).Foreground(lipgloss.Color("231")).Padding(0, 1)
	modeStyleInsert = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("71")).Foreground(lipgloss.Color("231")).Padding(0, 1)
	modeStyleVisual = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("176")).Foreground(lipgloss.Color("231")).Padding(0, 1)
	modeStyleWindow = lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("221")).Foreground(lipgloss.Color("235")).Padding(0, 1)
)

func zoneColor(z Zone) lipgloss.Color {
	switch z {
	case ZoneHeader:
		return colBlue
	case ZoneEditor:
		return colRed
	default:
		return colGreen
	}
}

// heights (total, border included) of each zone
func (a *App) zoneHeights() (int, int, int) {
	usable := a.height - 1 // status bar
	if usable < 12 {
		usable = 12
	}
	headerH := 3
	if a.focused && a.zone == ZoneHeader {
		headerH = 7
	}
	rest := usable - headerH
	var edH int
	switch {
	case a.focused && a.zone == ZoneEditor:
		edH = rest * 2 / 3
	case a.focused && a.zone == ZoneResults:
		edH = rest / 3
	default:
		edH = rest / 2
	}
	edH = clampInt(edH, 4, rest-4)
	resH := rest - edH
	return headerH, edH, resH
}

// borderStyleFor returns border + title styles depending on selection/focus.
func (a *App) borderStyleFor(z Zone) (lipgloss.Style, lipgloss.Style) {
	if a.zone == z {
		c := colAccent // selected in window-selection mode
		if a.focused {
			c = zoneColor(z) // focused
		}
		return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c),
			lipgloss.NewStyle().Foreground(c).Bold(true)
	}
	return dimBorder, dimText
}

func (a *App) layout() {
	if a.width == 0 || a.height == 0 {
		return
	}
	_, edH, resH := a.zoneHeights()
	innerW := maxInt(10, a.width-4)
	a.ed.SetSize(innerW, edH-2)
	a.res.SetSize(innerW, resH-2)
	a.ed.SetFocused(a.focused && a.zone == ZoneEditor)
	a.res.SetFocused(a.focused && a.zone == ZoneResults)
	if a.modal != nil {
		a.modal.SetSize(a.modalWidth(), a.modalHeight())
	}
	if a.form != nil {
		a.form.SetSize(a.modalWidth(), a.modalHeight())
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func (a *App) headerView(totalH int) string {
	w := maxInt(10, a.width-4)
	focused := a.focused && a.zone == ZoneHeader

	border, _ := a.borderStyleFor(ZoneHeader)

	labels := []string{i18n.T("header.connection"), i18n.T("header.database"), i18n.T("header.schema")}
	values := []string{orDash(a.connName), orDash(a.database), orDash(a.schema)}

	if !focused {
		segs := make([]string, 3)
		for i := range labels {
			segs[i] = dimText.Render(labels[i]+": ") + accText.Render(values[i])
		}
		line := truncLine(strings.Join(segs, dimText.Render("  │  ")), w)
		return border.Width(w + 2).Render(" " + line)
	}

	// focused: three boxes side by side, selected one highlighted
	boxW := (w - 6) / 3
	boxes := make([]string, 3)
	for i := range labels {
		bs := dimBorder
		lab := dimText.Render(labels[i])
		if i == a.headerSel {
			bs = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colAccent)
			lab = accText.Bold(true).Render(labels[i])
		}
		content := lab + "\n" + truncLine(values[i], boxW-2)
		boxes[i] = bs.Width(boxW).Render(content)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, boxes...)
	hint := dimText.Render(i18n.T("header.hint"))
	return border.Width(w + 2).Render(lipgloss.JoinVertical(lipgloss.Left, row, hint))
}

func (a *App) zoneBox(z Zone, title, content string, totalH int) string {
	w := maxInt(10, a.width-4)
	style, titleStyle := a.borderStyleFor(z)
	// title embedded on first line
	box := style.Width(w + 2).Height(totalH - 2).Render(content)
	// overlay the title into the top border
	lines := strings.Split(box, "\n")
	if len(lines) > 0 {
		t := " " + title + " "
		lines[0] = overlayTitle(lines[0], titleStyle.Render(t), 3)
	}
	return strings.Join(lines, "\n")
}

// overlayTitle inserts the rendered title into a border line at col offset.
func overlayTitle(borderLine, title string, offset int) string {
	// borderLine contains ANSI codes; simplest reliable approach: rebuild.
	w := lipgloss.Width(borderLine)
	tw := lipgloss.Width(title)
	if tw+offset+1 >= w {
		return borderLine
	}
	// Strip and rebuild the top border without styling loss is complex;
	// instead, re-render a plain top border and inject the title.
	left := strings.Repeat("─", offset)
	right := strings.Repeat("─", maxInt(0, w-offset-tw-2))
	return dimTopBorder("╭" + left) + title + dimTopBorder(right+"╮")
}

func dimTopBorder(s string) string {
	return lipgloss.NewStyle().Foreground(colDim).Render(s)
}

func (a *App) View() string {
	if a.width == 0 {
		return i18n.T("status.loading")
	}
	headerH, edH, resH := a.zoneHeights()

	header := a.headerView(headerH)

	edTitle := i18n.T("editor.title")
	editorBox := a.zoneBox(ZoneEditor, edTitle, a.ed.View(), edH)

	resTitle := i18n.T("results.title")
	if a.resInfo != "" {
		resTitle += " · " + a.resInfo
	}
	resultsBox := a.zoneBox(ZoneResults, resTitle, a.res.View(), resH)

	main := lipgloss.JoinVertical(lipgloss.Left, header, editorBox, resultsBox, a.statusBar())

	// help overlay (?)
	if a.showHelp != helpNone {
		content := resultsHelp()
		if a.showHelp == helpWindow {
			content = windowHelp()
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colGreen).
			Padding(0, 2).
			Render(content)
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
	}

	// modal overlay
	if a.form != nil || a.modal != nil {
		var inner string
		if a.form != nil {
			inner = a.form.View()
		} else {
			inner = a.modal.View()
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1).
			Width(a.modalWidth() + 2).
			Render(inner)
		return lipgloss.Place(a.width, a.height, lipgloss.Center, lipgloss.Center, box)
	}
	return main
}

// resultsHelp is the plain-text cheat sheet shown with "?" in the results zone.
func resultsHelp() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(colGreen)
	section := lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	lines := []string{
		title.Render(i18n.T("results.help.title")),
		"",
		section.Render(i18n.T("results.help.navigation")),
		i18n.T("results.help.nav.move"),
		i18n.T("results.help.nav.first"),
		i18n.T("results.help.nav.word"),
		i18n.T("results.help.nav.line"),
		i18n.T("results.help.nav.halfpage"),
		i18n.T("results.help.nav.page"),
		i18n.T("results.help.nav.screen"),
		i18n.T("results.help.nav.count"),
		"",
		section.Render(i18n.T("results.help.copy")),
		i18n.T("results.help.copy.cell"),
		i18n.T("results.help.copy.cellcol"),
		i18n.T("results.help.copy.row"),
		i18n.T("results.help.copy.rowhead"),
		i18n.T("results.help.copy.column"),
		i18n.T("results.help.copy.columnhead"),
		i18n.T("results.help.copy.all"),
		"",
		section.Render(i18n.T("results.help.search")),
		i18n.T("results.help.search.start"),
		i18n.T("results.help.search.next"),
		"",
		section.Render(i18n.T("results.help.misc")),
		i18n.T("results.help.misc.esc"),
		"",
		dimText.Render(i18n.T("results.help.close")),
	}
	return strings.Join(lines, "\n")
}

// windowHelp is the cheat sheet shown with "?" in window-selection mode.
func windowHelp() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(colGreen)
	section := lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	lines := []string{
		title.Render(i18n.T("window.help.title")),
		"",
		section.Render(i18n.T("window.help.navigation")),
		i18n.T("window.help.nav.move"),
		i18n.T("window.help.nav.focus"),
		i18n.T("window.help.nav.cycle"),
		"",
		section.Render(i18n.T("window.help.global")),
		i18n.T("window.help.global.exec"),
		i18n.T("window.help.global.history"),
		i18n.T("window.help.global.actions"),
		i18n.T("window.help.global.config"),
		i18n.T("window.help.global.quit"),
		"",
		dimText.Render(i18n.T("results.help.close")),
	}
	return strings.Join(lines, "\n")
}

func (a *App) statusBar() string {
	inModal := a.form != nil || a.modal != nil
	var mode string
	var modeStyle lipgloss.Style
	switch {
	case a.form != nil && a.form.InsertMode(),
		a.modal != nil && a.modal.InsertMode():
		mode = i18n.T("mode.insert")
		modeStyle = modeStyleInsert
	case inModal:
		mode = i18n.T("mode.normal")
		modeStyle = modeStyleNormal
	case !a.focused:
		mode = i18n.T("mode.window")
		modeStyle = modeStyleWindow
	case a.zone == ZoneEditor && a.ed.Mode() == editor.Insert:
		mode = i18n.T("mode.insert")
		modeStyle = modeStyleInsert
	case a.zone == ZoneEditor && a.ed.Mode() == editor.Visual:
		mode = i18n.T("mode.visual")
		modeStyle = modeStyleVisual
	case a.zone == ZoneEditor && a.ed.Mode() == editor.VisualLine:
		mode = i18n.T("mode.vline")
		modeStyle = modeStyleVisual
	default:
		mode = i18n.T("mode.normal")
		modeStyle = modeStyleNormal
	}
	left := modeStyle.Render(mode)

	pending := ""
	if a.focused && !inModal {
		if a.zone == ZoneEditor {
			pending = a.ed.PendingKeys()
		} else if a.zone == ZoneResults {
			pending = a.res.PendingKeys()
		}
	}
	if pending != "" {
		left += " " + accText.Render(pending)
	}

	msg := a.status
	st := statusOK
	if a.statusErr {
		st = statusBad
	}
	if a.busy {
		msg = "⏳ " + msg
	}

	hints := i18n.T("statusbar.hints.default")
	switch {
	case inModal:
		hints = i18n.T("statusbar.hints.modal")
	case !a.focused:
		hints = i18n.T("statusbar.hints.window")
	case a.zone == ZoneResults:
		hints = i18n.T("statusbar.hints.results") + hints
	}

	space := a.width - lipgloss.Width(left) - lipgloss.Width(msg) - len(hints) - 4
	if space < 1 {
		hints = ""
		space = maxInt(1, a.width-lipgloss.Width(left)-lipgloss.Width(msg)-2)
	}
	return left + " " + st.Render(msg) + strings.Repeat(" ", space) + dimText.Render(hints)
}
