package model

import "time"

type WebrootEnvVar struct {
	ID        string    `json:"id"`
	WebrootID string    `json:"webroot_id"`
	Name      string    `json:"name"`
	Value     string    `json:"value"`
	IsSecret  bool      `json:"is_secret"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
