// Package backup dumps and restores every table wholesale, behind the
// export and import subcommands. It reads raw rows rather than typed
// catalog models, so every table backs up the same way.
package backup

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

// tableOrder is foreign-key dependency order, parents first. Import
// deletes in reverse and reloads in this order, so every reference is
// satisfied as it is written.
var tableOrder = []string{
	"category",
	"concept",
	"month_entry",
	"note",
	"fx_rate",
	"settings",
}

// Data keys each table's rows by table name, each row by column name.
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

// Snapshot copies the database beside dbPath, timestamped so repeated
// snapshots never collide, and returns the path it wrote.
func Snapshot(db *sql.DB, dbPath string) (string, error) {
	dest := fmt.Sprintf("%s.%s.bak", dbPath, time.Now().UTC().Format("20060102T150405.000000000Z"))
	if _, err := db.Exec("VACUUM INTO ?", dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Import replaces every table with data.Tables and nothing else. There is
// no version guard: a row shaped for a schema this binary does not have
// fails its INSERT and rolls the whole import back.
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

// insertRow sorts columns only so a failed statement is deterministic to
// read; placeholders make the order irrelevant to correctness.
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

// table is always a tableOrder constant, never caller-supplied, so the
// interpolation below cannot carry injected SQL.
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
