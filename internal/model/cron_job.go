package model

import "time"

type CronJob struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	WebrootID        string    `json:"webroot_id"`
	Name             string    `json:"name"`
	Schedule         string    `json:"schedule"`
	Command          string    `json:"command"`
	WorkingDirectory string    `json:"working_directory"`
	Enabled          bool      `json:"enabled"`
	TimeoutSeconds   int       `json:"timeout_seconds"`
	MaxMemoryMB      int       `json:"max_memory_mb"`
	Status           string    `json:"status"`
	StatusMessage    *string   `json:"status_message,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
