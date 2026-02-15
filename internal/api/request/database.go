package request

type CreateDatabase struct {
	ShardID string                     `json:"shard_id" validate:"required"`
	Users   []CreateDatabaseUserNested `json:"users" validate:"omitempty,dive"`
}

type ReassignDatabaseTenant struct {
	TenantID *string `json:"tenant_id"`
}
