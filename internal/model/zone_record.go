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
	SourceType   string  `json:"source_type" db:"source_type"`
	SourceFQDNID *string `json:"source_fqdn_id,omitempty" db:"source_fqdn_id"`
	Status        string    `json:"status" db:"status"`
	StatusMessage *string   `json:"status_message,omitempty" db:"status_message"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// ZoneRecordParams contains all data needed by zone record workflows.
// Passed directly from the service layer to avoid an extra activity call.
type ZoneRecordParams struct {
	RecordID  string `json:"record_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Content   string `json:"content"`
	TTL       int    `json:"ttl"`
	Priority  *int   `json:"priority,omitempty"`
	ManagedBy string `json:"managed_by"`
	ZoneName  string `json:"zone_name"`
}

const (
	ManagedByCustom   = "custom"
	ManagedByAuto     = "auto"

	// Source types for auto-managed records.
	SourceTypeFQDN      = "fqdn"
	SourceTypeEmailMX   = "email-mx"
	SourceTypeEmailSPF  = "email-spf"
	SourceTypeEmailDKIM = "email-dkim"
	SourceTypeEmailDMARC      = "email-dmarc"
	SourceTypeServiceHostname = "service-hostname"
)
