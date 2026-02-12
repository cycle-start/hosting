package model

import (
	"encoding/json"
	"time"
)

type NodeProfile struct {
	ID          string          `json:"id" db:"id"`
	Name        string          `json:"name" db:"name"`
	Role        string          `json:"role" db:"role"`
	Image       string          `json:"image" db:"image"`
	Env         json.RawMessage `json:"env" db:"env"`
	Volumes     json.RawMessage `json:"volumes" db:"volumes"`
	Ports       json.RawMessage `json:"ports" db:"ports"`
	Resources   json.RawMessage `json:"resources" db:"resources"`
	HealthCheck json.RawMessage `json:"health_check" db:"health_check"`
	Privileged  bool            `json:"privileged" db:"privileged"`
	NetworkMode string          `json:"network_mode" db:"network_mode"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}
