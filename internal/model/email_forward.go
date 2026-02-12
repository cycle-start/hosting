package model

import "time"

type EmailForward struct {
	ID             string    `json:"id" db:"id"`
	EmailAccountID string    `json:"email_account_id" db:"email_account_id"`
	Destination    string    `json:"destination" db:"destination"`
	KeepCopy       bool      `json:"keep_copy" db:"keep_copy"`
	Status         string    `json:"status" db:"status"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}
