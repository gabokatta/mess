package main

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mes/internal/tui"
)

func main() {
	if _, err := tea.NewProgram(tui.New()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "mes:", err)
		os.Exit(1)
	}
}
