package model

import (
	"encoding/json"
	"time"
)

type Webroot struct {
	ID             string          `json:"id" db:"id"`
	TenantID       string          `json:"tenant_id" db:"tenant_id"`
	SubscriptionID string          `json:"subscription_id" db:"subscription_id"`
	Name           string          `json:"name" db:"name"`
	Runtime        string          `json:"runtime" db:"runtime"`
	RuntimeVersion string          `json:"runtime_version" db:"runtime_version"`
	RuntimeConfig  json.RawMessage `json:"runtime_config" db:"runtime_config"`
	PublicFolder   string          `json:"public_folder" db:"public_folder"`
	EnvFileName            string          `json:"env_file_name" db:"env_file_name"`
	ServiceHostnameEnabled bool            `json:"service_hostname_enabled" db:"service_hostname_enabled"`
	Status                 string          `json:"status" db:"status"`
	StatusMessage  *string         `json:"status_message,omitempty" db:"status_message"`
	SuspendReason  string          `json:"suspend_reason" db:"suspend_reason"`
	CreatedAt      time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at" db:"updated_at"`
}
