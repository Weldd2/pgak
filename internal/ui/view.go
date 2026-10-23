package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

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

	modeStyles = map[string]lipgloss.Style{
		"NORMAL":  lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("68")).Foreground(lipgloss.Color("231")).Padding(0, 1),
		"INSERT":  lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("71")).Foreground(lipgloss.Color("231")).Padding(0, 1),
		"VISUAL":  lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("176")).Foreground(lipgloss.Color("231")).Padding(0, 1),
		"V-LINE":  lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("176")).Foreground(lipgloss.Color("231")).Padding(0, 1),
		"FENÊTRE": lipgloss.NewStyle().Bold(true).Background(lipgloss.Color("221")).Foreground(lipgloss.Color("235")).Padding(0, 1),
	}
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

	labels := []string{"Connexion", "Base", "Schéma"}
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
	hint := dimText.Render(" h/l choisir · ↵ ouvrir · esc fenêtres")
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
		return "chargement…"
	}
	headerH, edH, resH := a.zoneHeights()

	header := a.headerView(headerH)

	edTitle := "Requête (ctrl+e exécuter · ctrl+p historique)"
	editorBox := a.zoneBox(ZoneEditor, edTitle, a.ed.View(), edH)

	resTitle := "Résultats"
	if a.resInfo != "" {
		resTitle += " · " + a.resInfo
	}
	resultsBox := a.zoneBox(ZoneResults, resTitle, a.res.View(), resH)

	main := lipgloss.JoinVertical(lipgloss.Left, header, editorBox, resultsBox, a.statusBar())

	// help overlay (?)
	if a.showHelp {
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colGreen).
			Padding(0, 2).
			Render(resultsHelp())
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
	key := accText
	lines := []string{
		title.Render("Résultats — aide"),
		"",
		section.Render("Navigation"),
		"  " + key.Render("h j k l / flèches") + "  se déplacer (cellule)",
		"  " + key.Render("0 / $            ") + "  première / dernière colonne",
		"  " + key.Render("w / b            ") + "  colonne suivante / précédente",
		"  " + key.Render("gg / G           ") + "  première / dernière ligne (42G, 5gg : ligne n)",
		"  " + key.Render("ctrl+d / ctrl+u  ") + "  demi-page bas / haut",
		"  " + key.Render("ctrl+f / ctrl+b  ") + "  page bas / haut",
		"  " + key.Render("H / M / L        ") + "  haut / milieu / bas de l'écran",
		"  " + key.Render("10j, 3l, …       ") + "  compteurs sur les mouvements",
		"",
		section.Render("Copier (presse-papier système)"),
		"  " + key.Render("yc") + "  cellule",
		"  " + key.Render("yC") + "  cellule avec nom de colonne (col: valeur)",
		"  " + key.Render("yy") + "  ligne courante (TSV)",
		"  " + key.Render("yY") + "  ligne courante + en-têtes",
		"  " + key.Render("yo") + "  colonne entière",
		"  " + key.Render("yO") + "  colonne entière + en-tête",
		"  " + key.Render("ya") + "  tout le résultat (TSV avec en-têtes)",
		"",
		section.Render("Divers"),
		"  " + key.Render("esc") + "  revenir au mode fenêtres    " + key.Render("q") + "  quitter",
		"",
		dimText.Render("appuyez sur une touche pour fermer"),
	}
	return strings.Join(lines, "\n")
}

func (a *App) statusBar() string {
	inModal := a.form != nil || a.modal != nil
	mode := "NORMAL"
	switch {
	case a.form != nil && a.form.InsertMode(),
		a.modal != nil && a.modal.InsertMode():
		mode = "INSERT"
	case inModal:
		mode = "NORMAL"
	case !a.focused:
		mode = "FENÊTRE"
	case a.zone == ZoneEditor && a.ed.Mode() == editor.Insert:
		mode = "INSERT"
	case a.zone == ZoneEditor && a.ed.Mode() == editor.Visual:
		mode = "VISUAL"
	case a.zone == ZoneEditor && a.ed.Mode() == editor.VisualLine:
		mode = "V-LINE"
	}
	left := modeStyles[mode].Render(mode)

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

	hints := "esc fenêtres · ctrl+t actions · ctrl+c quitter"
	switch {
	case inModal:
		hints = "ctrl+t actions · ctrl+c quitter"
	case !a.focused:
		hints = "hjkl naviguer · ↵ focus · ctrl+t actions · q quitter"
	case a.zone == ZoneResults:
		hints = "? aide · y y/Y/c/C/o/O/a copier · " + hints
	}

	space := a.width - lipgloss.Width(left) - lipgloss.Width(msg) - len(hints) - 4
	if space < 1 {
		hints = ""
		space = maxInt(1, a.width-lipgloss.Width(left)-lipgloss.Width(msg)-2)
	}
	return left + " " + st.Render(msg) + strings.Repeat(" ", space) + dimText.Render(hints)
}
