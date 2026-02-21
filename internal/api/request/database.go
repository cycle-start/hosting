package request

type CreateDatabase struct {
	SubscriptionID string                     `json:"subscription_id" validate:"required"`
	ShardID        string                     `json:"shard_id" validate:"required"`
	Users          []CreateDatabaseUserNested `json:"users" validate:"omitempty,dive"`
}
