package model

import "time"

// DatabaseAccessRule represents a per-database network access restriction.
// Rules control which source CIDRs can connect to a database via MySQL host patterns.
type DatabaseAccessRule struct {
	ID            string    `json:"id" db:"id"`
	DatabaseID    string    `json:"database_id" db:"database_id"`
	CIDR          string    `json:"cidr" db:"cidr"`
	Description   string    `json:"description" db:"description"`
	Status        string    `json:"status" db:"status"`
	StatusMessage *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}
