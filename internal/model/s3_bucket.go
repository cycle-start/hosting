package model

import "time"

type S3Bucket struct {
	ID         string  `json:"id" db:"id"`
	TenantID   *string `json:"tenant_id,omitempty" db:"tenant_id"`
	Name       string  `json:"name" db:"name"`
	ShardID    *string `json:"shard_id,omitempty" db:"shard_id"`
	Public     bool    `json:"public" db:"public"`
	QuotaBytes int64   `json:"quota_bytes" db:"quota_bytes"`
	Status     string  `json:"status" db:"status"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}
