package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/gabokatta/mess/internal/backup"
	"github.com/gabokatta/mess/internal/store"
	"github.com/gabokatta/mess/internal/tui"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "mess:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "export":
			return runExport(args[1:], stdout)
		case "import":
			return runImport(args[1:])
		}
	}
	return runTUI(args)
}

func runTUI(args []string) error {
	fs := flag.NewFlagSet("mess", flag.ExitOnError)
	dbPath := fs.String("db", "", "database path (default: user config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := resolveDBPath(*dbPath)
	if err != nil {
		return err
	}

	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()

	_, err = tea.NewProgram(tui.New(s.DB()).WithDBPath(path)).Run()
	return err
}

func runExport(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("mess export", flag.ExitOnError)
	dbPath := fs.String("db", "", "database path (default: user config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := resolveDBPath(*dbPath)
	if err != nil {
		return err
	}

	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()

	data, err := backup.Export(s.DB())
	if err != nil {
		return err
	}
	return json.NewEncoder(stdout).Encode(data)
}

func runImport(args []string) error {
	fs := flag.NewFlagSet("mess import", flag.ExitOnError)
	dbPath := fs.String("db", "", "database path (default: user config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: mess import <file>")
	}

	raw, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	var data backup.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}

	path, err := resolveDBPath(*dbPath)
	if err != nil {
		return err
	}

	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()

	if _, err := backup.Snapshot(s.DB(), path); err != nil {
		return err
	}
	return backup.Import(s.DB(), data)
}

func resolveDBPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return store.DefaultPath()
}
