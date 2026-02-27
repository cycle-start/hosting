package model

import "time"

type Product struct {
	ID          string    `json:"id"`
	BrandID     string    `json:"brand_id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Modules     []string  `json:"modules"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
