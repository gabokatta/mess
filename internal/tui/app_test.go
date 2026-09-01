package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func key(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
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
			var m tea.Model = New()
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
	var m tea.Model = New()
	_, cmd := m.Update(key("q"))
	if cmd == nil {
		t.Fatal("q returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("q returned %T, want tea.QuitMsg", cmd())
	}
}

func TestViewDeclaresTerminalState(t *testing.T) {
	m := New()
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
