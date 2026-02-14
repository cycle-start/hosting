package model

import "time"

type EmailAccount struct {
	ID          string    `json:"id" db:"id"`
	FQDNID      string    `json:"fqdn_id" db:"fqdn_id"`
	Address     string    `json:"address" db:"address"`
	DisplayName string    `json:"display_name" db:"display_name"`
	QuotaBytes  int64     `json:"quota_bytes" db:"quota_bytes"`
	Status        string    `json:"status" db:"status"`
	StatusMessage *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
