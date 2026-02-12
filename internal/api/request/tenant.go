package request

type CreateTenant struct {
	Name        string `json:"name" validate:"required,slug"`
	RegionID    string `json:"region_id" validate:"required"`
	ClusterID   string `json:"cluster_id" validate:"required"`
	ShardID     string `json:"shard_id" validate:"required"`
	SFTPEnabled *bool  `json:"sftp_enabled"`
}

type UpdateTenant struct {
	SFTPEnabled *bool `json:"sftp_enabled"`
}
