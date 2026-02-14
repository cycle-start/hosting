package model

import "time"

type DatabaseUser struct {
	ID         string   `json:"id" db:"id"`
	DatabaseID string   `json:"database_id" db:"database_id"`
	Username   string   `json:"username" db:"username"`
	Password   string   `json:"password,omitempty" db:"password"`
	Privileges []string `json:"privileges" db:"privileges"`
	Status        string   `json:"status" db:"status"`
	StatusMessage *string  `json:"status_message,omitempty" db:"status_message"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
