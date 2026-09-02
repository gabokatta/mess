package catalog

import "database/sql"

type Category struct {
	ID        int64
	Name      string
	SortOrder int
}

func CreateCategory(db *sql.DB, name string, sortOrder int) (Category, error) {
	res, err := db.Exec(`INSERT INTO category (name, sort_order) VALUES (?, ?)`, name, sortOrder)
	if err != nil {
		return Category{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Category{}, err
	}
	return Category{ID: id, Name: name, SortOrder: sortOrder}, nil
}

func Categories(db *sql.DB) ([]Category, error) {
	rows, err := db.Query(`SELECT id, name, sort_order FROM category ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.SortOrder); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// FindOrCreateCategory returns the category named name, creating it —
// appended after every existing one — if none exists yet. The match is
// exact, so the concept form never leaves the category picker up to fuzzy
// matching.
func FindOrCreateCategory(db *sql.DB, name string) (Category, error) {
	categories, err := Categories(db)
	if err != nil {
		return Category{}, err
	}
	for _, c := range categories {
		if c.Name == name {
			return c, nil
		}
	}
	return CreateCategory(db, name, len(categories))
}

// DefaultCategoryNames seeds a new database's category picker so it isn't
// starting from nothing. FindOrCreateCategory covers anything beyond them.
var DefaultCategoryNames = []string{"Earnings", "Home", "Utilities", "Cards", "Other"}

// EnsureDefaultCategories creates DefaultCategoryNames if the category
// table is still empty. It never runs against a table that already has
// rows, so it's safe to call on every app start.
func EnsureDefaultCategories(db *sql.DB) error {
	categories, err := Categories(db)
	if err != nil {
		return err
	}
	if len(categories) > 0 {
		return nil
	}
	for i, name := range DefaultCategoryNames {
		if _, err := CreateCategory(db, name, i); err != nil {
			return err
		}
	}
	return nil
}

func UpdateCategory(db *sql.DB, c Category) error {
	_, err := db.Exec(`UPDATE category SET name = ?, sort_order = ? WHERE id = ?`, c.Name, c.SortOrder, c.ID)
	return err
}
