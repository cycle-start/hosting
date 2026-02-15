package request

import (
	"encoding/json"
)

type CreateWebroot struct {
	Runtime        string             `json:"runtime" validate:"required,oneof=php node python ruby static"`
	RuntimeVersion string             `json:"runtime_version" validate:"required"`
	RuntimeConfig  json.RawMessage    `json:"runtime_config"`
	PublicFolder   string             `json:"public_folder"`
	FQDNs          []CreateFQDNNested `json:"fqdns" validate:"omitempty,dive"`
}

type UpdateWebroot struct {
	Runtime        string          `json:"runtime" validate:"omitempty,oneof=php node python ruby static"`
	RuntimeVersion string          `json:"runtime_version"`
	RuntimeConfig  json.RawMessage `json:"runtime_config"`
	PublicFolder   *string         `json:"public_folder"`
}
