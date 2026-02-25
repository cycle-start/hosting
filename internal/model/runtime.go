package model

import (
	"encoding/json"
	"time"
)

type ClusterRuntime struct {
	ClusterID string `json:"cluster_id" db:"cluster_id"`
	Runtime   string `json:"runtime" db:"runtime"`
	Version   string `json:"version" db:"version"`
	Available bool   `json:"available" db:"available"`
}

type TenantRuntimeConfig struct {
	ID         string          `json:"id" db:"id"`
	TenantID   string          `json:"tenant_id" db:"tenant_id"`
	Runtime    string          `json:"runtime" db:"runtime"`
	PoolConfig json.RawMessage `json:"pool_config" db:"pool_config"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

const (
	RuntimePHP    = "php"
	RuntimeNode   = "node"
	RuntimePython = "python"
	RuntimeRuby   = "ruby"
	RuntimeStatic = "static"
)
