package model

import (
	"encoding/json"
	"time"
)

type Region struct {
	ID        string          `json:"id" db:"id"`
	Name      string          `json:"name" db:"name"`
	Config    json.RawMessage `json:"config" db:"config"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}
