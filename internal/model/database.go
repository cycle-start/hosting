package model

import "time"

type Database struct {
	ID        string  `json:"id" db:"id"`
	TenantID  *string `json:"tenant_id,omitempty" db:"tenant_id"`
	Name      string  `json:"name" db:"name"`
	ShardID   *string `json:"shard_id,omitempty" db:"shard_id"`
	NodeID    *string `json:"node_id,omitempty" db:"node_id"`
	Status        string  `json:"status" db:"status"`
	StatusMessage *string `json:"status_message,omitempty" db:"status_message"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}
