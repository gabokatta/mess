package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/gabokatta/mess/internal/backup"
	"github.com/gabokatta/mess/internal/catalog"
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
			return runImport(args[1:], stdout, confirmReplace)
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

	_, err = tea.NewProgram(tui.New(s.DB())).Run()
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
	if err := json.NewEncoder(stdout).Encode(data); err != nil {
		return err
	}
	return catalog.MarkExported(s.DB(), time.Now())
}

// confirm gates the wholesale replace. It is a parameter so a test can
// answer it; the only real implementation is confirmReplace.
type confirm func(dbPath string) (bool, error)

func runImport(args []string, stdout io.Writer, ask confirm) error {
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

	confirmed, err := ask(path)
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	snapshot, err := backup.Snapshot(s.DB(), path)
	if err != nil {
		return err
	}
	if err := backup.Import(s.DB(), data); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "imported; the database as it was is at", snapshot)
	return nil
}

// confirmReplace is the gate itself. Import is a recovery action run by
// hand, not the crontab half of backup, so it can ask.
func confirmReplace(dbPath string) (bool, error) {
	var confirmed bool
	err := huh.NewConfirm().
		Title("Replace every table in " + dbPath + " with this backup?").
		Description("A timestamped copy of the current database is written first.").
		Affirmative("Replace").Negative("Cancel").
		Value(&confirmed).
		Run()
	return confirmed, err
}

func resolveDBPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return store.DefaultPath()
}
