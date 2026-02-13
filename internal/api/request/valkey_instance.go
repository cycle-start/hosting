package request

type CreateValkeyInstance struct {
	Name        string                   `json:"name" validate:"required,slug"`
	ShardID     string                   `json:"shard_id" validate:"required"`
	MaxMemoryMB int                      `json:"max_memory_mb" validate:"omitempty,min=1"`
	Users       []CreateValkeyUserNested `json:"users" validate:"omitempty,dive"`
}

type ReassignValkeyInstanceTenant struct {
	TenantID *string `json:"tenant_id"`
}
