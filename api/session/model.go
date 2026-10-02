package session

import "time"

type Session struct {
	TokenHash string    `json:"-"` // The "-" means do not include in JSON output
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}
