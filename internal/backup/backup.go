// Package backup dumps and restores every table wholesale. It reads raw rows
// rather than typed catalog models, so every table backs up the same way.
package backup

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Foreign-key dependency order, parents first; import deletes in reverse.
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

func Decode(r io.Reader) (Data, error) {
	decoder := json.NewDecoder(r)
	decoder.UseNumber()
	decoder.DisallowUnknownFields()
	var data Data
	if err := decoder.Decode(&data); err != nil {
		return Data{}, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Data{}, fmt.Errorf("backup: expected one JSON document")
	}
	return data, nil
}

func Export(db *sql.DB) (Data, error) {
	tx, err := db.Begin()
	if err != nil {
		return Data{}, err
	}
	defer tx.Rollback()

	data := Data{Tables: make(map[string][]map[string]any, len(tableOrder))}
	for _, table := range tableOrder {
		rows, err := dumpTable(tx, table)
		if err != nil {
			return Data{}, err
		}
		data.Tables[table] = rows
	}
	return data, tx.Commit()
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

// Import replaces every table with data.Tables. There is no version guard: a
// row shaped for an unknown schema fails its INSERT and rolls the lot back.
func Import(db *sql.DB, data Data) error {
	for _, table := range tableOrder {
		if _, ok := data.Tables[table]; !ok {
			return fmt.Errorf("backup: missing table %q", table)
		}
	}
	if len(data.Tables) != len(tableOrder) {
		return fmt.Errorf("backup: contains unknown tables")
	}

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

func insertRow(tx *sql.Tx, table string, row map[string]any) error {
	cols := make([]string, 0, len(row))
	for c := range row {
		cols = append(cols, c)
	}
	sort.Strings(cols)

	vals := make([]any, len(cols))
	quoted := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		vals[i] = row[c]
		if number, ok := vals[i].(json.Number); ok {
			value, err := number.Int64()
			if err != nil {
				return fmt.Errorf("backup: %s.%s: %w", table, c, err)
			}
			vals[i] = value
		}
		quoted[i] = `"` + strings.ReplaceAll(c, `"`, `""`) + `"`
		placeholders[i] = "?"
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", table, strings.Join(quoted, ", "), strings.Join(placeholders, ", "))
	_, err := tx.Exec(query, vals...)
	return err
}

// table is always a tableOrder constant, never caller-supplied, so the
// interpolation cannot carry injected SQL.
func dumpTable(tx *sql.Tx, table string) ([]map[string]any, error) {
	rows, err := tx.Query("SELECT * FROM " + table)
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
