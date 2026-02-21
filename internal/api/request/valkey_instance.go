package request

type CreateValkeyInstance struct {
	SubscriptionID string                   `json:"subscription_id" validate:"required"`
	ShardID        string                   `json:"shard_id" validate:"required"`
	MaxMemoryMB    int                      `json:"max_memory_mb" validate:"omitempty,min=1"`
	Users          []CreateValkeyUserNested `json:"users" validate:"omitempty,dive"`
}
