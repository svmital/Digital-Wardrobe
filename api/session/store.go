package session

import "database/sql"

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(session Session) (Session, error) {
	err := store.db.QueryRow(`
		INSERT INTO sessions (token_hash, user_id)
		VALUES ($1, $2)
		RETURNING created_at, expires_at
	`, session.TokenHash, session.UserID).Scan(
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		return Session{}, err
	}

	return session, nil
}

func (store *Store) FindByTokenHash(tokenHash string) (Session, error) {
	var session Session

	err := store.db.QueryRow(`
		SELECT token_hash, user_id, created_at, expires_at
		FROM sessions
		WHERE token_hash = $1
		AND expires_at > NOW()
	`, tokenHash).Scan(
		&session.TokenHash,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		return Session{}, err
	}

	return session, nil
}

func (store *Store) DeleteByTokenHash(tokenHash string) error {
	_, err := store.db.Exec("DELETE FROM sessions WHERE token_hash = $1", tokenHash)
	return err
}
