package model

import "time"

type Tenant struct {
	ID          string  `json:"id" db:"id"`
	RegionID    string  `json:"region_id" db:"region_id"`
	ClusterID   string  `json:"cluster_id" db:"cluster_id"`
	ShardID     *string `json:"shard_id,omitempty" db:"shard_id"`
	UID         int     `json:"uid" db:"uid"`
	SFTPEnabled bool    `json:"sftp_enabled" db:"sftp_enabled"`
	Status      string  `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// ResourceStatusCounts maps status strings to their counts.
type ResourceStatusCounts map[string]int

// TenantResourceSummary aggregates resource counts grouped by status
// for all resources belonging to a tenant.
type TenantResourceSummary struct {
	Webroots        ResourceStatusCounts `json:"webroots"`
	FQDNs           ResourceStatusCounts `json:"fqdns"`
	Certificates    ResourceStatusCounts `json:"certificates"`
	EmailAccounts   ResourceStatusCounts `json:"email_accounts"`
	EmailAliases    ResourceStatusCounts `json:"email_aliases"`
	EmailForwards   ResourceStatusCounts `json:"email_forwards"`
	EmailAutoReplies ResourceStatusCounts `json:"email_autoreplies"`
	Databases       ResourceStatusCounts `json:"databases"`
	DatabaseUsers   ResourceStatusCounts `json:"database_users"`
	Zones           ResourceStatusCounts `json:"zones"`
	ZoneRecords     ResourceStatusCounts `json:"zone_records"`
	ValkeyInstances ResourceStatusCounts `json:"valkey_instances"`
	ValkeyUsers     ResourceStatusCounts `json:"valkey_users"`
	SFTPKeys        ResourceStatusCounts `json:"sftp_keys"`
	Backups         ResourceStatusCounts `json:"backups"`
	Total           int                  `json:"total"`
	Pending         int                  `json:"pending"`
	Provisioning    int                  `json:"provisioning"`
	Failed          int                  `json:"failed"`
}
