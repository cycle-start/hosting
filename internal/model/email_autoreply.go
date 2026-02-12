package model

import "time"

type EmailAutoReply struct {
	ID             string     `json:"id" db:"id"`
	EmailAccountID string     `json:"email_account_id" db:"email_account_id"`
	Subject        string     `json:"subject" db:"subject"`
	Body           string     `json:"body" db:"body"`
	StartDate      *time.Time `json:"start_date,omitempty" db:"start_date"`
	EndDate        *time.Time `json:"end_date,omitempty" db:"end_date"`
	Enabled        bool       `json:"enabled" db:"enabled"`
	Status         string     `json:"status" db:"status"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
}
