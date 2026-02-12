package model

import (
	"encoding/json"
	"time"
)

type Webroot struct {
	ID             string          `json:"id" db:"id"`
	TenantID       string          `json:"tenant_id" db:"tenant_id"`
	Name           string          `json:"name" db:"name"`
	Runtime        string          `json:"runtime" db:"runtime"`
	RuntimeVersion string          `json:"runtime_version" db:"runtime_version"`
	RuntimeConfig  json.RawMessage `json:"runtime_config" db:"runtime_config"`
	PublicFolder   string          `json:"public_folder" db:"public_folder"`
	Status         string          `json:"status" db:"status"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}
