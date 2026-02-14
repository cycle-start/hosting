package model

import "time"

type EmailAlias struct {
	ID             string    `json:"id" db:"id"`
	EmailAccountID string    `json:"email_account_id" db:"email_account_id"`
	Address        string    `json:"address" db:"address"`
	Status         string    `json:"status" db:"status"`
	StatusMessage  *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}
