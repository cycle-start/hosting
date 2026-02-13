package request

import "encoding/json"

type CreateCluster struct {
	ID     string          `json:"id" validate:"required,slug"`
	Name   string          `json:"name" validate:"required,slug"`
	Config json.RawMessage `json:"config"`
	Spec   json.RawMessage `json:"spec"`
}

type UpdateCluster struct {
	Name   string          `json:"name"`
	Status string          `json:"status"`
	Config json.RawMessage `json:"config"`
	Spec   json.RawMessage `json:"spec"`
}
