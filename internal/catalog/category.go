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

func UpdateCategory(db *sql.DB, c Category) error {
	_, err := db.Exec(`UPDATE category SET name = ?, sort_order = ? WHERE id = ?`, c.Name, c.SortOrder, c.ID)
	return err
}
