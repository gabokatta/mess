package main

import (
	"flag"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mess/internal/store"
	"github.com/gabokatta/mess/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mess:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "database path (default: user config dir)")
	flag.Parse()

	path := *dbPath
	if path == "" {
		p, err := store.DefaultPath()
		if err != nil {
			return err
		}
		path = p
	}

	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()

	_, err = tea.NewProgram(tui.New(s.DB())).Run()
	return err
}
