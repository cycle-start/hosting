package model

import "time"

type Tenant struct {
	ID          string  `json:"id" db:"id"`
	BrandID     string  `json:"brand_id" db:"brand_id"`
	RegionID    string  `json:"region_id" db:"region_id"`
	ClusterID   string  `json:"cluster_id" db:"cluster_id"`
	ShardID     *string `json:"shard_id,omitempty" db:"shard_id"`
	UID         int     `json:"uid" db:"uid"`
	SFTPEnabled    bool    `json:"sftp_enabled" db:"sftp_enabled"`
	SSHEnabled     bool    `json:"ssh_enabled" db:"ssh_enabled"`
	DiskQuotaBytes int64   `json:"disk_quota_bytes" db:"disk_quota_bytes"`
	Status         string  `json:"status" db:"status"`
	StatusMessage *string `json:"status_message,omitempty" db:"status_message"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
	RegionName  string    `json:"region_name,omitempty" db:"-"`
	ClusterName string    `json:"cluster_name,omitempty" db:"-"`
	ShardName   *string   `json:"shard_name,omitempty" db:"-"`
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
	SSHKeys         ResourceStatusCounts `json:"ssh_keys"`
	Backups         ResourceStatusCounts `json:"backups"`
	CronJobs        ResourceStatusCounts `json:"cron_jobs"`
	Total           int                  `json:"total"`
	Pending         int                  `json:"pending"`
	Provisioning    int                  `json:"provisioning"`
	Failed          int                  `json:"failed"`
}
