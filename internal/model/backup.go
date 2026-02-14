package model

import "time"

type Backup struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	Type        string     `json:"type"`
	SourceID    string     `json:"source_id"`
	SourceName  string     `json:"source_name"`
	StoragePath string     `json:"storage_path,omitempty"`
	SizeBytes   int64      `json:"size_bytes"`
	Status        string     `json:"status"`
	StatusMessage *string    `json:"status_message,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

const (
	BackupTypeWeb      = "web"
	BackupTypeDatabase = "database"
)
