package model

import "time"

type ResourceUsage struct {
	ID           string    `json:"id"`
	ResourceType string    `json:"resource_type"`
	ResourceID   string    `json:"resource_id"`
	TenantID     string    `json:"tenant_id"`
	BytesUsed    int64     `json:"bytes_used"`
	CollectedAt  time.Time `json:"collected_at"`
}
