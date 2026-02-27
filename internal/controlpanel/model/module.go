package model

import "time"

type BrandModules struct {
	BrandID         string    `json:"brand_id"`
	DisabledModules []string  `json:"disabled_modules"`
	UpdatedAt       time.Time `json:"updated_at"`
}
