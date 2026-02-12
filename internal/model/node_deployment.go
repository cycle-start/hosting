package model

import (
	"encoding/json"
	"time"
)

type NodeDeployment struct {
	ID            string          `json:"id" db:"id"`
	NodeID        string          `json:"node_id" db:"node_id"`
	HostMachineID string          `json:"host_machine_id" db:"host_machine_id"`
	ProfileID     string          `json:"profile_id" db:"profile_id"`
	ContainerID   string          `json:"container_id" db:"container_id"`
	ContainerName string          `json:"container_name" db:"container_name"`
	ImageDigest   string          `json:"image_digest" db:"image_digest"`
	EnvOverrides  json.RawMessage `json:"env_overrides" db:"env_overrides"`
	Status        string          `json:"status" db:"status"`
	DeployedAt    *time.Time      `json:"deployed_at,omitempty" db:"deployed_at"`
	LastHealthAt  *time.Time      `json:"last_health_at,omitempty" db:"last_health_at"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at" db:"updated_at"`
}
