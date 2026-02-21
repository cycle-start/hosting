package model

import "time"

type CronExecution struct {
	ID         string    `json:"id"`
	CronJobID  string    `json:"cron_job_id"`
	NodeID     string    `json:"node_id"`
	Success    bool      `json:"success"`
	ExitCode   *int      `json:"exit_code,omitempty"`
	DurationMs *int      `json:"duration_ms,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	CreatedAt  time.Time `json:"created_at"`
}

type CronJob struct {
	ID                  string    `json:"id"`
	TenantID            string    `json:"tenant_id"`
	WebrootID           string    `json:"webroot_id"`
	Name                string    `json:"name"`
	Schedule            string    `json:"schedule"`
	Command             string    `json:"command"`
	WorkingDirectory    string    `json:"working_directory"`
	Enabled             bool      `json:"enabled"`
	TimeoutSeconds      int       `json:"timeout_seconds"`
	MaxMemoryMB         int       `json:"max_memory_mb"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	MaxFailures         int       `json:"max_failures"`
	Status              string    `json:"status"`
	StatusMessage       *string   `json:"status_message,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}
