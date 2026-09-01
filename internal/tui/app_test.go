package tui

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/shopspring/decimal"

	"github.com/gabokatta/mes/internal/catalog"
	"github.com/gabokatta/mes/internal/domain"
	"github.com/gabokatta/mes/internal/month"
	"github.com/gabokatta/mes/internal/store"
)

func key(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

func keySpace() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "} }
func keyEnter() tea.KeyPressMsg { return tea.KeyPressMsg{Code: tea.KeyEnter} }
func keyEsc() tea.KeyPressMsg   { return tea.KeyPressMsg{Code: tea.KeyEscape} }

// settle drains a Cmd/Update chain to completion, the way the real Bubble
// Tea loop would deliver each Cmd's Msg back into Update.
func settle(t *testing.T, m tea.Model, cmd tea.Cmd) Model {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		m, cmd = m.Update(msg)
	}
	return m.(Model)
}

func openTestStore(t *testing.T) *sql.DB {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "mes.db"))
	if err != nil {
		t.Fatalf("store.Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s.DB()
}

func TestViewCycling(t *testing.T) {
	tests := []struct {
		name string
		keys []tea.KeyPressMsg
		want view
	}{
		{"starts on month", nil, viewMonth},
		{"tab advances", []tea.KeyPressMsg{key("l")}, viewYear},
		{"wraps forward", []tea.KeyPressMsg{key("l"), key("l"), key("l"), key("l"), key("l")}, viewMonth},
		{"wraps backward", []tea.KeyPressMsg{key("h")}, viewSettings},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m tea.Model = New(openTestStore(t))
			for _, k := range tt.keys {
				m, _ = m.Update(k)
			}
			if got := m.(Model).view; got != tt.want {
				t.Errorf("view = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestQuitOnQ(t *testing.T) {
	var m tea.Model = New(openTestStore(t))
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q returned %T, want tea.QuitMsg", cmd())
	}
}

func TestViewDeclaresTerminalState(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40
	v := m.View()

	if !v.AltScreen {
		t.Error("AltScreen = false, want true")
	}
	if v.MouseMode != tea.MouseModeNone {
		t.Errorf("MouseMode = %v, want MouseModeNone (native selection must keep working)", v.MouseMode)
	}
	if !strings.Contains(v.Content, "Month") {
		t.Error("content does not name the focused view")
	}
}

func TestMonthViewRendersGroupedLines(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	rent := catalog.Concept{Name: "Alquiler", Kind: catalog.FixedExpense, Currency: domain.ARS}
	salary := catalog.Concept{Name: "Sueldo", Kind: catalog.Income, Currency: domain.ARS}
	lines := []month.Line{
		{Concept: salary, Amount: amountFor(t, "450000"), Confirmed: true, Done: true},
		{Concept: rent, Amount: amountFor(t, "785000"), Confirmed: false, Done: false},
	}

	updated, _ := m.Update(monthLoadedMsg{lines: lines})
	m = updated.(Model)
	content := m.View().Content

	for _, want := range []string{"Income", "Sueldo", "FixedExpense", "Alquiler", "projected"} {
		if !strings.Contains(content, want) {
			t.Errorf("month view content missing %q:\n%s", want, content)
		}
	}
}

func TestMonthViewReportsLoadError(t *testing.T) {
	m := New(openTestStore(t))
	m.width, m.height = 100, 40

	updated, _ := m.Update(monthLoadedMsg{err: sql.ErrConnDone})
	m = updated.(Model)
	content := m.View().Content

	if !strings.Contains(content, "failed to load") {
		t.Errorf("month view content = %q, want it to surface the load error", content)
	}
}

func amountFor(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatalf("decimal.NewFromString(%q) unexpected error: %v", s, err)
	}
	return d
}
