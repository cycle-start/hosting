package model

import "time"

type Partner struct {
	ID           string    `json:"id"`
	BrandID      string    `json:"brand_id"`
	Name         string    `json:"name"`
	Hostname     string    `json:"hostname"`
	PrimaryColor string    `json:"primary_color"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
