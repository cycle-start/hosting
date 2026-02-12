package model

import "time"

type ZoneRecord struct {
	ID           string  `json:"id" db:"id"`
	ZoneID       string  `json:"zone_id" db:"zone_id"`
	Type         string  `json:"type" db:"type"`
	Name         string  `json:"name" db:"name"`
	Content      string  `json:"content" db:"content"`
	TTL          int     `json:"ttl" db:"ttl"`
	Priority     *int    `json:"priority,omitempty" db:"priority"`
	ManagedBy    string  `json:"managed_by" db:"managed_by"`
	SourceFQDNID *string `json:"source_fqdn_id,omitempty" db:"source_fqdn_id"`
	Status       string  `json:"status" db:"status"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

const (
	ManagedByUser     = "user"
	ManagedByPlatform = "platform"
)
