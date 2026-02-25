package request

import "encoding/json"

type CreateRegion struct {
	Name   string          `json:"name" validate:"required,slug"`
	Config json.RawMessage `json:"config"`
}

type UpdateRegion struct {
	Name   string          `json:"name"`
	Config json.RawMessage `json:"config"`
}

type AddClusterRuntime struct {
	Runtime string `json:"runtime" validate:"required"`
	Version string `json:"version" validate:"required"`
}

type RemoveClusterRuntime struct {
	Runtime string `json:"runtime" validate:"required"`
	Version string `json:"version" validate:"required"`
}
