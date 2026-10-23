package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/anmura/pgak/internal/config"
)

// ConnForm is a classic form to create/edit a connection.
// Normal mode: j/k move between fields, i/enter edits a field, ctrl+s saves,
// esc closes. Insert mode: type, esc back to normal.
type ConnForm struct {
	OldName string // empty = new connection
	fields  []formField
	cursor  int
	insert  bool

	width, height int

	titleStyle lipgloss.Style
	labStyle   lipgloss.Style
	curStyle   lipgloss.Style
	dimStyle   lipgloss.Style
	errStyle   lipgloss.Style

	Err string
}

type formField struct {
	label       string
	value       string
	placeholder string // dim example shown when empty
	desc        string // help text shown when the field is selected
}

// FormResult is emitted when the form closes.
type FormResult struct {
	Canceled bool
	OldName  string
	Conn     config.Connection
}

func NewConnForm(old *config.Connection) *ConnForm {
	f := &ConnForm{
		titleStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		labStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("150")).Width(23),
		curStyle:   lipgloss.NewStyle().Reverse(true),
		dimStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		errStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("203")),
	}
	c := config.Connection{Port: 5432, SSLMode: "prefer"}
	if old != nil {
		c = *old
		f.OldName = old.Name
	}
	f.fields = []formField{
		{"Nom", c.Name, "prod",
			"Nom affiché dans la liste des connexions"},
		{"Host", c.Host, "localhost",
			"Hôte du serveur postgres (container docker : vérifier le port mappé)"},
		{"Port", strconv.Itoa(defaultPort(c.Port)), "5432",
			"Port TCP (docker : le port *externe*, ex. 5434 pour 5434->5432)"},
		{"User", c.User, "postgres",
			"Utilisateur postgres"},
		{"Mot de passe (var env)", c.PasswordEnv, "PGAK_PROD_PASSWORD",
			"⚠ PAS le mot de passe ! Le NOM d'une variable d'env qui le contient.\n   Ex: PGAK_PROD_PASSWORD puis → fish: set -x PGAK_PROD_PASSWORD monmdp\n   (bash/zsh: export PGAK_PROD_PASSWORD=monmdp) · vide = pas de mot de passe"},
		{"Database", c.Database, "postgres",
			"Base par défaut à l'ouverture (vide = postgres)"},
		{"SSL mode", defaultStr(c.SSLMode, "prefer"), "prefer",
			"disable · prefer · require · verify-ca · verify-full"},
	}
	return f
}

func defaultPort(p int) int {
	if p == 0 {
		return 5432
	}
	return p
}

func defaultStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func (f *ConnForm) SetSize(w, h int) { f.width, f.height = w, h }

func (f *ConnForm) InsertMode() bool { return f.insert }

func (f *ConnForm) connection() (config.Connection, string) {
	port, err := strconv.Atoi(strings.TrimSpace(f.fields[2].value))
	if err != nil || port <= 0 {
		return config.Connection{}, "port invalide"
	}
	c := config.Connection{
		Name:        strings.TrimSpace(f.fields[0].value),
		Host:        strings.TrimSpace(f.fields[1].value),
		Port:        port,
		User:        strings.TrimSpace(f.fields[3].value),
		PasswordEnv: strings.TrimSpace(f.fields[4].value),
		Database:    strings.TrimSpace(f.fields[5].value),
		SSLMode:     strings.TrimSpace(f.fields[6].value),
	}
	if c.Name == "" {
		return c, "le nom est requis"
	}
	if c.Host == "" {
		return c, "le host est requis"
	}
	if c.User == "" {
		return c, "le user est requis"
	}
	return c, ""
}

// Update handles a key; returns a non-nil result when the form should close.
func (f *ConnForm) Update(msg tea.KeyMsg) *FormResult {
	key := msg.String()
	f.Err = ""

	if key == "ctrl+s" {
		c, errMsg := f.connection()
		if errMsg != "" {
			f.Err = errMsg
			return nil
		}
		return &FormResult{OldName: f.OldName, Conn: c}
	}

	if f.insert {
		fld := &f.fields[f.cursor]
		switch key {
		case "esc":
			f.insert = false
		case "enter", "tab", "down":
			f.insert = false
			f.cursor = (f.cursor + 1) % len(f.fields)
			f.insert = true
		case "shift+tab", "up":
			f.cursor = (f.cursor - 1 + len(f.fields)) % len(f.fields)
		case "backspace":
			if len(fld.value) > 0 {
				rs := []rune(fld.value)
				fld.value = string(rs[:len(rs)-1])
			}
		default:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				if msg.Type == tea.KeySpace {
					fld.value += " "
				} else {
					fld.value += string(msg.Runes)
				}
			}
		}
		return nil
	}

	switch key {
	case "esc", "q":
		return &FormResult{Canceled: true}
	case "j", "down", "tab":
		f.cursor = (f.cursor + 1) % len(f.fields)
	case "k", "up", "shift+tab":
		f.cursor = (f.cursor - 1 + len(f.fields)) % len(f.fields)
	case "g":
		f.cursor = 0
	case "G":
		f.cursor = len(f.fields) - 1
	case "i", "a", "enter":
		f.insert = true
	case "d":
		f.fields[f.cursor].value = ""
	}
	return nil
}

func (f *ConnForm) View() string {
	var sb strings.Builder
	title := "Nouvelle connexion"
	if f.OldName != "" {
		title = "Édition : " + f.OldName
	}
	sb.WriteString(f.titleStyle.Render(title))
	sb.WriteString("\n\n")

	for i, fld := range f.fields {
		marker := "  "
		val := fld.value
		if val == "" && !(i == f.cursor && f.insert) {
			val = f.dimStyle.Render(fld.placeholder)
		}
		if i == f.cursor {
			marker = f.curStyle.Render("▸") + " "
			if f.insert {
				val = fld.value + f.curStyle.Render(" ")
			} else if fld.value != "" {
				val = f.curStyle.Render(fld.value)
			}
		}
		sb.WriteString(marker + f.labStyle.Render(fld.label) + " " + val)
		sb.WriteByte('\n')
	}

	// description of the selected field
	sb.WriteByte('\n')
	sb.WriteString(f.dimStyle.Render("ℹ " + f.fields[f.cursor].desc))
	sb.WriteByte('\n')

	sb.WriteByte('\n')
	if f.Err != "" {
		sb.WriteString(f.errStyle.Render("✗ " + f.Err))
		sb.WriteByte('\n')
	}
	hint := "j/k naviguer · i éditer · ctrl+s sauvegarder · esc fermer"
	if f.insert {
		hint = "esc mode normal · ↵/tab champ suivant · ctrl+s sauvegarder"
	}
	sb.WriteString(f.dimStyle.Render(hint))
	return sb.String()
}
