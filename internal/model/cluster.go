package model

import (
	"encoding/json"
	"time"
)

type Cluster struct {
	ID        string          `json:"id" db:"id"`
	RegionID  string          `json:"region_id" db:"region_id"`
	Name      string          `json:"name" db:"name"`
	Config    json.RawMessage `json:"config" db:"config"`
	Status    string          `json:"status" db:"status"`
	Spec      json.RawMessage `json:"spec" db:"spec"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}
