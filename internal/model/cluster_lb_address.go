package model

import "time"

type ClusterLBAddress struct {
	ID        string    `json:"id" db:"id"`
	ClusterID string    `json:"cluster_id" db:"cluster_id"`
	Address   string    `json:"address" db:"address"`
	Family    int       `json:"family" db:"family"`
	Label     string    `json:"label" db:"label"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
