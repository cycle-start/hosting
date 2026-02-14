package model

import "time"

type FQDN struct {
	ID         string    `json:"id" db:"id"`
	FQDN       string    `json:"fqdn" db:"fqdn"`
	WebrootID  string    `json:"webroot_id" db:"webroot_id"`
	SSLEnabled bool      `json:"ssl_enabled" db:"ssl_enabled"`
	Status        string    `json:"status" db:"status"`
	StatusMessage *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
