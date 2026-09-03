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
	"github.com/gabokatta/mess/internal/domain"
	"github.com/gabokatta/mess/internal/fixture"
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
		case "seed":
			return runSeed(args[1:], stdout)
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

func runSeed(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("mess seed", flag.ExitOnError)
	dbPath := fs.String("db", "", "database path (default: user config dir)")
	periodFlag := fs.String("period", "", "pin the anchor month, YYYY-MM (default: the current month)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path, err := resolveDBPath(*dbPath)
	if err != nil {
		return err
	}

	anchor, err := resolveAnchor(*periodFlag)
	if err != nil {
		return err
	}

	if err := removeDatabaseFiles(path); err != nil {
		return err
	}

	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer s.Close()

	loaded, err := fixture.Load(s.DB(), fixture.Demo(anchor))
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "seeded %s: anchor %s, %d concepts, %d categories, %d notes\n",
		path, anchor, len(loaded.Concepts), len(loaded.Categories), len(loaded.Notes))
	return nil
}

func resolveAnchor(override string) (domain.Period, error) {
	if override != "" {
		return domain.ParsePeriod(override)
	}
	return domain.PeriodFromTime(time.Now()), nil
}

// Exact names rather than a glob, since path's directory also holds snapshots.
func removeDatabaseFiles(path string) error {
	for _, suffix := range []string{"", "-wal", "-shm", ".lock"} {
		if err := os.Remove(path + suffix); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("seed: remove %s: %w", path+suffix, err)
		}
	}
	return nil
}

func resolveDBPath(override string) (string, error) {
	if override != "" {
		return override, nil
	}
	return store.DefaultPath()
}
