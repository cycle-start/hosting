package model

import "time"

// APIKey represents an API key for authenticating against the platform API.
type APIKey struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	KeyHash   string     `json:"-"`
	KeyPrefix string     `json:"key_prefix,omitempty"`
	Scopes    []string   `json:"scopes"`
	Brands    []string   `json:"brands"`
	CreatedAt time.Time  `json:"created_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}
