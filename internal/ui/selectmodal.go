package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"pgak/internal/i18n"
)

// SelectItem is an entry of a searchable select list.
type SelectItem struct {
	Label string // displayed and matched text
	Desc  string // optional dimmed description
	Value any    // arbitrary payload
}

// SelectModal is a fuzzy-searchable list with vim bindings.
// Insert mode: typing filters; esc -> normal mode.
// Normal mode: j/k navigate, enter selects, esc closes, i or / back to search.
type SelectModal struct {
	Title string
	items []SelectItem

	query    string
	filtered []int // indexes into items
	cursor   int
	scroll   int
	insert   bool

	width, height int

	// Extra actions shown in the footer and reported via SelectResult.Action
	// when the matching key is pressed in normal mode (e.g. "e" edit, "n" new).
	Actions map[string]string // key -> label

	titleStyle lipgloss.Style
	curStyle   lipgloss.Style
	dimStyle   lipgloss.Style
	qStyle     lipgloss.Style
}

// SelectResult is emitted when the modal closes.
type SelectResult struct {
	Canceled bool
	Action   string // "" = normal selection, otherwise an action key
	Item     SelectItem
	HasItem  bool
}

func NewSelectModal(title string, items []SelectItem) *SelectModal {
	m := &SelectModal{
		Title:      title,
		items:      items,
		insert:     true,
		titleStyle: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("81")),
		curStyle:   lipgloss.NewStyle().Reverse(true),
		dimStyle:   lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		qStyle:     lipgloss.NewStyle().Foreground(lipgloss.Color("221")),
	}
	m.filter()
	return m
}

func (m *SelectModal) SetSize(w, h int) { m.width, m.height = w, h }

func (m *SelectModal) InsertMode() bool { return m.insert }

func fuzzyMatch(query, s string) bool {
	query = strings.ToLower(query)
	s = strings.ToLower(s)
	i := 0
	for _, r := range s {
		if i < len(query) && rune(query[i]) == r {
			i++
		}
	}
	return i == len(query)
}

func (m *SelectModal) filter() {
	m.filtered = m.filtered[:0]
	for i, it := range m.items {
		if m.query == "" || fuzzyMatch(m.query, it.Label) || fuzzyMatch(m.query, it.Desc) {
			m.filtered = append(m.filtered, i)
		}
	}
	m.cursor = clampInt(m.cursor, 0, maxInt(0, len(m.filtered)-1))
	m.scroll = 0
}

func (m *SelectModal) current() (SelectItem, bool) {
	if len(m.filtered) == 0 {
		return SelectItem{}, false
	}
	return m.items[m.filtered[m.cursor]], true
}

// Update handles a key; returns a non-nil result when the modal should close.
func (m *SelectModal) Update(msg tea.KeyMsg) *SelectResult {
	key := msg.String()

	// keys valid in both modes
	switch key {
	case "enter":
		if it, ok := m.current(); ok {
			return &SelectResult{Item: it, HasItem: true}
		}
		return &SelectResult{Canceled: true}
	case "ctrl+c":
		return &SelectResult{Canceled: true}
	case "down", "ctrl+n", "ctrl+j":
		m.move(1)
		return nil
	case "up", "ctrl+p", "ctrl+k":
		m.move(-1)
		return nil
	}

	if m.insert {
		switch key {
		case "esc":
			m.insert = false
		case "backspace":
			if len(m.query) > 0 {
				rs := []rune(m.query)
				m.query = string(rs[:len(rs)-1])
				m.filter()
			}
		default:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				if msg.Type == tea.KeySpace {
					m.query += " "
				} else {
					m.query += string(msg.Runes)
				}
				m.filter()
			}
		}
		return nil
	}

	// normal mode
	switch key {
	case "esc", "q":
		return &SelectResult{Canceled: true}
	case "j":
		m.move(1)
	case "k":
		m.move(-1)
	case "g":
		m.cursor = 0
		m.ensureVisible()
	case "G":
		m.cursor = maxInt(0, len(m.filtered)-1)
		m.ensureVisible()
	case "i", "/", "a":
		m.insert = true
	default:
		if m.Actions != nil {
			if _, ok := m.Actions[key]; ok {
				it, has := m.current()
				return &SelectResult{Action: key, Item: it, HasItem: has}
			}
		}
	}
	return nil
}

func (m *SelectModal) move(d int) {
	m.cursor = clampInt(m.cursor+d, 0, maxInt(0, len(m.filtered)-1))
	m.ensureVisible()
}

func (m *SelectModal) listHeight() int {
	return maxInt(1, m.height-4) // title + search + sep + footer
}

func (m *SelectModal) ensureVisible() {
	lh := m.listHeight()
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+lh {
		m.scroll = m.cursor - lh + 1
	}
}

func (m *SelectModal) View() string {
	var sb strings.Builder
	sb.WriteString(m.titleStyle.Render(m.Title))
	sb.WriteByte('\n')

	prompt := "/ "
	cursor := " "
	mode := m.dimStyle.Render(i18n.T("modal.normal"))
	if m.insert {
		cursor = m.curStyle.Render(" ")
		mode = m.qStyle.Render(i18n.T("modal.search"))
	}
	sb.WriteString(prompt + m.query + cursor + " " + mode)
	sb.WriteByte('\n')
	sb.WriteString(m.dimStyle.Render(strings.Repeat("─", maxInt(0, m.width))))

	lh := m.listHeight()
	for i := 0; i < lh; i++ {
		idx := m.scroll + i
		sb.WriteByte('\n')
		if idx >= len(m.filtered) {
			continue
		}
		it := m.items[m.filtered[idx]]
		line := it.Label
		if it.Desc != "" {
			line += "  " + m.dimStyle.Render(it.Desc)
		}
		line = truncLine(line, m.width-2)
		if idx == m.cursor {
			sb.WriteString(m.curStyle.Render("▸ ") + line)
		} else {
			sb.WriteString("  " + line)
		}
	}

	sb.WriteByte('\n')
	foot := i18n.T("modal.hint.select")
	if m.insert {
		foot += i18n.T("modal.hint.normalmode")
	} else {
		foot += i18n.T("modal.hint.close")
	}
	if m.Actions != nil && !m.insert {
		for k, label := range m.Actions {
			foot += " · " + k + " " + label
		}
	}
	sb.WriteString(m.dimStyle.Render(truncLine(foot, m.width)))
	return sb.String()
}
