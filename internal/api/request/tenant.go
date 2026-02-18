package request

type CreateTenant struct {
	BrandID        string `json:"brand_id" validate:"required"`
	RegionID       string `json:"region_id" validate:"required"`
	ClusterID      string `json:"cluster_id" validate:"required"`
	ShardID        string `json:"shard_id" validate:"required"`
	SFTPEnabled    *bool  `json:"sftp_enabled"`
	SSHEnabled     *bool  `json:"ssh_enabled"`
	DiskQuotaBytes *int64 `json:"disk_quota_bytes"`
	// Nested (all optional)
	Zones           []CreateZoneNested           `json:"zones" validate:"omitempty,dive"`
	Webroots        []CreateWebrootNested        `json:"webroots" validate:"omitempty,dive"`
	Databases       []CreateDatabaseNested       `json:"databases" validate:"omitempty,dive"`
	ValkeyInstances []CreateValkeyInstanceNested `json:"valkey_instances" validate:"omitempty,dive"`
	S3Buckets       []CreateS3BucketNested       `json:"s3_buckets" validate:"omitempty,dive"`
	SSHKeys         []CreateSSHKeyNested         `json:"ssh_keys" validate:"omitempty,dive"`
	EgressRules     []CreateEgressRuleNested     `json:"egress_rules" validate:"omitempty,dive"`
}

type UpdateTenant struct {
	SFTPEnabled    *bool  `json:"sftp_enabled"`
	SSHEnabled     *bool  `json:"ssh_enabled"`
	DiskQuotaBytes *int64 `json:"disk_quota_bytes"`
}
