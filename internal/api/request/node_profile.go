package request

import "encoding/json"

type CreateNodeProfile struct {
	Name        string          `json:"name" validate:"required,slug"`
	Role        string          `json:"role" validate:"required,oneof=web database dns valkey email"`
	Image       string          `json:"image" validate:"required"`
	Env         json.RawMessage `json:"env"`
	Volumes     json.RawMessage `json:"volumes"`
	Ports       json.RawMessage `json:"ports"`
	Resources   json.RawMessage `json:"resources"`
	HealthCheck json.RawMessage `json:"health_check"`
	Privileged  *bool           `json:"privileged"`
	NetworkMode string          `json:"network_mode"`
}

type UpdateNodeProfile struct {
	Name        string          `json:"name"`
	Role        string          `json:"role" validate:"omitempty,oneof=web database dns valkey email"`
	Image       string          `json:"image"`
	Env         json.RawMessage `json:"env"`
	Volumes     json.RawMessage `json:"volumes"`
	Ports       json.RawMessage `json:"ports"`
	Resources   json.RawMessage `json:"resources"`
	HealthCheck json.RawMessage `json:"health_check"`
	Privileged  *bool           `json:"privileged"`
	NetworkMode string          `json:"network_mode"`
}
