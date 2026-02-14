package model

import "time"

type Zone struct {
	ID        string  `json:"id" db:"id"`
	BrandID   string  `json:"brand_id" db:"brand_id"`
	TenantID  *string `json:"tenant_id,omitempty" db:"tenant_id"`
	Name      string  `json:"name" db:"name"`
	RegionID  string  `json:"region_id" db:"region_id"`
	Status        string  `json:"status" db:"status"`
	StatusMessage *string `json:"status_message,omitempty" db:"status_message"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
