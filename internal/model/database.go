package model

import "time"

type Database struct {
	ID             string  `json:"id" db:"id"`
	TenantID       string  `json:"tenant_id" db:"tenant_id"`
	SubscriptionID string  `json:"subscription_id" db:"subscription_id"`
	ShardID        *string `json:"shard_id,omitempty" db:"shard_id"`
	NodeID         *string `json:"node_id,omitempty" db:"node_id"`
	Status         string  `json:"status" db:"status"`
	StatusMessage  *string `json:"status_message,omitempty" db:"status_message"`
	SuspendReason  string  `json:"suspend_reason" db:"suspend_reason"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
	ShardName      *string   `json:"shard_name,omitempty" db:"-"`
}
