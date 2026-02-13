package request

type CreateDatabase struct {
	Name    string                     `json:"name" validate:"required,slug"`
	ShardID string                     `json:"shard_id" validate:"required"`
	Users   []CreateDatabaseUserNested `json:"users" validate:"omitempty,dive"`
}

type ReassignDatabaseTenant struct {
	TenantID *string `json:"tenant_id"`
}
