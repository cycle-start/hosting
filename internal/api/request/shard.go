package request

import "encoding/json"

type CreateShard struct {
	Name      string          `json:"name" validate:"required,slug"`
	Role      string          `json:"role" validate:"required,oneof=web database dns valkey email storage dbadmin lb"`
	LBBackend string          `json:"lb_backend"`
	Config    json.RawMessage `json:"config"`
}

type UpdateShard struct {
	LBBackend string          `json:"lb_backend"`
	Config    json.RawMessage `json:"config"`
	Status    string          `json:"status"`
}
