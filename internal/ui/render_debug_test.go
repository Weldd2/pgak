package ui

import (
	"fmt"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// Debug rendering, run with: PGAK_RENDER=1 go test ./internal/ui -run TestPrintRender -v
func TestPrintRender(t *testing.T) {
	if os.Getenv("PGAK_RENDER") != "1" {
		t.Skip("PGAK_RENDER != 1")
	}
	a := newTestApp()
	a.connName, a.database, a.schema = "prod", "app", "public"
	a.res.SetData([]string{"id", "email", "created_at"}, [][]string{
		{"1", "alice@example.com", "2024-01-01"},
		{"2", "bob@example.com", "2024-02-01"},
	})
	a.resInfo = "2 ligne(s) · 3ms"
	a.ed.SetBanner("  SET search_path TO public;  -- appliqué automatiquement (schéma sélectionné)")
	press(a, "enter") // focus editor
	press(a, "i", "S", "E", "L", "E", "C", "T", " ", "*", "esc")
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 26})
	fmt.Println(a.View())
}

// Debug modal rendering, run with: PGAK_RENDER=1
func TestPrintModalRender(t *testing.T) {
	if os.Getenv("PGAK_RENDER") != "1" {
		t.Skip("PGAK_RENDER != 1")
	}
	a := newTestApp()
	a.Init()
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	fmt.Println(a.View())
}

// Debug results help overlay, run with: PGAK_RENDER=1
func TestPrintHelpRender(t *testing.T) {
	if os.Getenv("PGAK_RENDER") != "1" {
		t.Skip("PGAK_RENDER != 1")
	}
	a := newTestApp()
	a.res.SetData([]string{"id"}, [][]string{{"1"}})
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	press(a, "j", "enter", "?")
	fmt.Println(a.View())
}

// Debug form rendering, run with: PGAK_RENDER=1
func TestPrintFormRender(t *testing.T) {
	if os.Getenv("PGAK_RENDER") != "1" {
		t.Skip("PGAK_RENDER != 1")
	}
	a := newTestApp()
	a.form = NewConnForm(nil)
	a.form.SetSize(a.modalWidth(), a.modalHeight())
	press(a, "j", "j", "j", "j") // move to the password env field
	fmt.Println(a.View())
}
