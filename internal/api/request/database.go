package request

type CreateDatabase struct {
	Name    string `json:"name" validate:"required,slug"`
	ShardID string `json:"shard_id" validate:"required"`
}

type ReassignDatabaseTenant struct {
	TenantID *string `json:"tenant_id"`
}
