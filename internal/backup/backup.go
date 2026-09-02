// Package backup dumps and restores every table wholesale — the engine
// behind both the export/import CLI subcommands and the Settings actions
// that will call the same functions. It reads and writes raw rows, not
// typed catalog models, so a table with no Go model yet (settings) backs
// up and restores exactly like any other.
package backup

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// tableOrder is every table in foreign-key dependency order, parents
// first. Export writes them in this order for a readable file; Import
// deletes them in reverse and reloads them in this order so every
// reference is satisfied as it's written.
var tableOrder = []string{
	"category",
	"concept",
	"base_amount",
	"month_entry",
	"saving_allocation",
	"list",
	"fx_rate",
	"settings",
}

// Data is a full database snapshot: every table's rows, keyed by table
// name, each row keyed by column name.
type Data struct {
	Tables map[string][]map[string]any `json:"tables"`
}

func Export(db *sql.DB) (Data, error) {
	data := Data{Tables: make(map[string][]map[string]any, len(tableOrder))}
	for _, table := range tableOrder {
		rows, err := dumpTable(db, table)
		if err != nil {
			return Data{}, err
		}
		data.Tables[table] = rows
	}
	return data, nil
}

// Snapshot writes a standalone, consistent copy of the database beside
// dbPath, timestamped so repeated snapshots never collide, and returns its
// path. Import calls this first — the irreversible-overwrite safety net a
// wholesale replace needs.
func Snapshot(db *sql.DB, dbPath string) (string, error) {
	dest := fmt.Sprintf("%s.%s.bak", dbPath, time.Now().UTC().Format("20060102T150405.000000000Z"))
	if _, err := db.Exec("VACUUM INTO ?", dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Import replaces every table wholesale: every row in data.Tables, and
// nothing else. There is no version guard — a row shaped for a schema
// Import doesn't have fails the INSERT, and the whole import rolls back.
// Like dumpTable, the table names it interpolates come only from
// tableOrder, never from data or a caller.
func Import(db *sql.DB, data Data) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i := len(tableOrder) - 1; i >= 0; i-- {
		if _, err := tx.Exec("DELETE FROM " + tableOrder[i]); err != nil {
			return err
		}
	}
	for _, table := range tableOrder {
		for _, row := range data.Tables[table] {
			if err := insertRow(tx, table, row); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// insertRow writes row's columns in sorted order — sorted only so the
// generated statement is deterministic to read in a failure, since named
// placeholders make the actual order irrelevant to correctness.
func insertRow(tx *sql.Tx, table string, row map[string]any) error {
	cols := make([]string, 0, len(row))
	for c := range row {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	vals := make([]any, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		vals[i] = row[c]
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	_, err := tx.Exec(query, vals...)
	return err
}

// dumpTable is a bounded, hardcoded lookup into tableOrder, never a caller-
// supplied string, so the interpolation below never carries injected SQL.
func dumpTable(db *sql.DB, table string) ([]map[string]any, error) {
	rows, err := db.Query("SELECT * FROM " + table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, c := range cols {
			row[c] = vals[i]
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
