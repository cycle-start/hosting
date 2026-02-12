package model

import "time"

type Tenant struct {
	ID          string  `json:"id" db:"id"`
	Name        string  `json:"name" db:"name"`
	RegionID    string  `json:"region_id" db:"region_id"`
	ClusterID   string  `json:"cluster_id" db:"cluster_id"`
	ShardID     *string `json:"shard_id,omitempty" db:"shard_id"`
	UID         int     `json:"uid" db:"uid"`
	SFTPEnabled bool    `json:"sftp_enabled" db:"sftp_enabled"`
	Status      string  `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
