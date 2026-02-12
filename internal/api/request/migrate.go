package request

type MigrateTenant struct {
	TargetShardID string `json:"target_shard_id" validate:"required"`
	MigrateZones  bool   `json:"migrate_zones"`
	MigrateFQDNs  bool   `json:"migrate_fqdns"`
}
