package wardrobe

import "database/sql"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) List(userID string) ([]WardrobeItem, error) {
	rows, err := store.db.Query(`
		SELECT id, user_id, name, category, color, image_url, source, created_at, updated_at
		FROM wardrobe_items
		WHERE user_id = $1
		ORDER BY id
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []WardrobeItem
	for rows.Next() {
		var item WardrobeItem
		err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Name,
			&item.Category,
			&item.Color,
			&item.ImageURL,
			&item.Source,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

func (store *Store) Create(userID string, item WardrobeItem) (WardrobeItem, error) {
	item.UserID = userID
	err := store.db.QueryRow(`
		INSERT INTO wardrobe_items (user_id, name, category, color, image_url, source)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, item.UserID, item.Name, item.Category, item.Color, item.ImageURL, item.Source).Scan(
		&item.ID,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return WardrobeItem{}, err
	}

	return item, nil
}

func (store *Store) Update(userID string, id string, item WardrobeItem) (WardrobeItem, error) {
	item.UserID = userID
	err := store.db.QueryRow(`
		UPDATE wardrobe_items
		SET name = $1, category = $2, color = $3, image_url = $4,
			source = $5, updated_at = NOW()
		WHERE id = $6 AND user_id = $7
		RETURNING id, created_at, updated_at
	`, item.Name, item.Category, item.Color, item.ImageURL, item.Source, id, userID).Scan(
		&item.ID,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return WardrobeItem{}, err
	}

	return item, nil
}

func (store *Store) Delete(userID string, id string) error {
	result, err := store.db.Exec("DELETE FROM wardrobe_items WHERE id = $1 AND user_id = $2", id, userID)
	if err != nil {
		return err
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if deleted == 0 {
		return sql.ErrNoRows
	}

	return nil
}
