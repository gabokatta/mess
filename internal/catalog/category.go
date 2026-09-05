package catalog

import (
	"database/sql"
	"fmt"
)

// PaletteSize is how many colours a category can be given. It lives here
// rather than in the TUI because the catalog assigns an index on create and
// has to know what it is allowed to assign.
const PaletteSize = 8

// Category's ColorIndex is a field rather than the row's position, so the
// order categories are listed in and the colour they render in are two
// separate facts. Two categories may share an index: the catalog can be
// larger than the palette, and every screen prints the name beside the hue.
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

// DeleteCategory refuses two things, and only one of them belongs to the
// schema. A category holding concepts is refused by the foreign key, which is
// referential integrity and always true, so this runs the delete and
// translates the refusal.
//
// The last category is different. An empty catalog is consistent data that the
// app cannot use, not corrupt data, and a restore legitimately passes through
// it: backup.Import empties every table and refills it in one transaction. A
// trigger would refuse that, so this rule is checked here, on the one path a
// person deletes a category by hand.
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

// RenameCategory translates the UNIQUE constraint rather than guarding ahead
// of it: the database owns the rule, and a check here would be a second copy
// of it, free to drift.
func RenameCategory(db *sql.DB, id int64, name string) error {
	_, err := db.Exec(`UPDATE category SET name = ? WHERE id = ?`, name, id)
	return explainNameClash(db, err, name, id)
}

// explainNameClash turns the UNIQUE constraint's refusal into a sentence. It
// runs only after the database has already said no, so it reports the rule
// rather than duplicating it. An error it cannot explain is passed through
// unchanged.
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

// SetCategoryColor is idempotent: writing the index a category already has
// changes nothing, which is what cycling through the palette needs.
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

// NextColorIndex is the lowest palette slot no category holds, and the least
// used one once every slot is taken. A collision has to land somewhere, and
// spreading them beats always colliding with the first category.
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

// AppendCategory adds a category at the end of the list with the next free
// colour. A duplicate name is refused by UNIQUE, and translated here.
func AppendCategory(db *sql.DB, name string) (Category, error) {
	categories, err := Categories(db)
	if err != nil {
		return Category{}, err
	}
	c, err := CreateCategory(db, name, len(categories), NextColorIndex(categories))
	if err != nil {
		// No row to exclude: nothing has this id, so every match is a clash.
		return Category{}, explainNameClash(db, err, name, 0)
	}
	return c, nil
}

// DefaultCategoryNames seeds a new database's category picker.
var DefaultCategoryNames = []string{"Earnings", "Home", "Utilities", "Cards", "Other"}

// EnsureDefaultCategories creates DefaultCategoryNames only when the table
// is empty, so it is safe on every app start.
func EnsureDefaultCategories(db *sql.DB) error {
	categories, err := Categories(db)
	if err != nil {
		return err
	}
	if len(categories) > 0 {
		return nil
	}
	for i, name := range DefaultCategoryNames {
		if _, err := CreateCategory(db, name, i, i%PaletteSize); err != nil {
			return err
		}
	}
	return nil
}
