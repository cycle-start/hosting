package model

import "time"

type ValkeyInstance struct {
	ID          string  `json:"id" db:"id"`
	TenantID    *string `json:"tenant_id,omitempty" db:"tenant_id"`
	Name        string  `json:"name" db:"name"`
	ShardID     *string `json:"shard_id,omitempty" db:"shard_id"`
	Port        int     `json:"port" db:"port"`
	MaxMemoryMB int     `json:"max_memory_mb" db:"max_memory_mb"`
	Password    string  `json:"password,omitempty" db:"password"`
	Status        string  `json:"status" db:"status"`
	StatusMessage *string `json:"status_message,omitempty" db:"status_message"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
