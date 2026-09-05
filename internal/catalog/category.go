package catalog

import (
	"database/sql"
	"fmt"
)

// PaletteSize matches the color_index constraint in the schema.
const PaletteSize = 8

// ColorIndex is independent of sort order and may be shared by categories.
type Category struct {
	ID         int64
	Name       string
	SortOrder  int
	ColorIndex int
}

func CreateCategory(db *sql.DB, name string, sortOrder, colorIndex int) (Category, error) {
	res, err := db.Exec(`INSERT INTO category (name, sort_order, color_index) VALUES (?, ?, ?)`,
		name, sortOrder, colorIndex)
	if err != nil {
		return Category{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Category{}, err
	}
	return Category{ID: id, Name: name, SortOrder: sortOrder, ColorIndex: colorIndex}, nil
}

// Preserve the last category here, not in a trigger: backup imports temporarily
// empty the table. Foreign keys prevent deleting categories that hold concepts.
func DeleteCategory(db *sql.DB, id int64) error {
	categories, err := Categories(db)
	if err != nil {
		return err
	}
	if len(categories) == 1 && categories[0].ID == id {
		return fmt.Errorf("%s is the last category, and concepts need one", categories[0].Name)
	}

	if _, err := db.Exec(`DELETE FROM category WHERE id = ?`, id); err != nil {
		if name, held, lookupErr := categoryHolding(db, id); lookupErr == nil && held > 0 {
			return fmt.Errorf("%s holds %d concepts", name, held)
		}
		return err
	}
	return nil
}

func categoryHolding(db *sql.DB, id int64) (name string, concepts int, err error) {
	err = db.QueryRow(`
		SELECT c.name, (SELECT COUNT(*) FROM concept WHERE category_id = c.id)
		FROM category c WHERE c.id = ?`, id).Scan(&name, &concepts)
	return name, concepts, err
}

func RenameCategory(db *sql.DB, id int64, name string) error {
	_, err := db.Exec(`UPDATE category SET name = ? WHERE id = ?`, name, id)
	return explainNameClash(db, err, name, id)
}

// Translate duplicate-name failures while preserving unrelated database errors.
func explainNameClash(db *sql.DB, err error, name string, excluding int64) error {
	if err == nil {
		return nil
	}

	var others int
	if lookup := db.QueryRow(
		`SELECT COUNT(*) FROM category WHERE name = ? AND id <> ?`, name, excluding,
	).Scan(&others); lookup != nil {
		return err
	}
	if others > 0 {
		return fmt.Errorf("a category named %q already exists", name)
	}
	return err
}

func SetCategoryColor(db *sql.DB, id int64, colorIndex int) error {
	_, err := db.Exec(`UPDATE category SET color_index = ? WHERE id = ?`, colorIndex, id)
	return err
}

func Categories(db *sql.DB) ([]Category, error) {
	rows, err := db.Query(`SELECT id, name, sort_order, color_index FROM category ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder, &c.ColorIndex); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// Pick the least-used color, breaking ties by palette order.
func NextColorIndex(categories []Category) int {
	used := make([]int, PaletteSize)
	for _, c := range categories {
		used[c.ColorIndex]++
	}
	lowest := 0
	for i, count := range used {
		if count < used[lowest] {
			lowest = i
		}
	}
	return lowest
}

func AppendCategory(db *sql.DB, name string) (Category, error) {
	categories, err := Categories(db)
	if err != nil {
		return Category{}, err
	}
	sortOrder := 0
	if len(categories) > 0 {
		sortOrder = categories[len(categories)-1].SortOrder + 1
	}
	c, err := CreateCategory(db, name, sortOrder, NextColorIndex(categories))
	if err != nil {
		// No row to exclude: nothing has this id, so every match is a clash.
		return Category{}, explainNameClash(db, err, name, 0)
	}
	return c, nil
}

// Seed only an empty catalog, preserving existing categories on restart.
func EnsureDefaultCategories(db *sql.DB) error {
	categories, err := Categories(db)
	if err != nil {
		return err
	}
	if len(categories) > 0 {
		return nil
	}
	for i, name := range []string{"Earnings", "Home", "Utilities", "Cards", "Other"} {
		if _, err := CreateCategory(db, name, i, i%PaletteSize); err != nil {
			return err
		}
	}
	return nil
}
