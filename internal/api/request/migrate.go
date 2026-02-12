package request

type MigrateTenant struct {
	TargetShardID string `json:"target_shard_id" validate:"required"`
	MigrateZones  bool   `json:"migrate_zones"`
	MigrateFQDNs  bool   `json:"migrate_fqdns"`
}

type MigrateDatabase struct {
	TargetShardID string `json:"target_shard_id" validate:"required"`
}

type MigrateValkeyInstance struct {
	TargetShardID string `json:"target_shard_id" validate:"required"`
}
