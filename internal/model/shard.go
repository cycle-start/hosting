package model

import (
	"encoding/json"
	"time"
)

type Shard struct {
	ID        string          `json:"id" db:"id"`
	ClusterID string          `json:"cluster_id" db:"cluster_id"`
	Name      string          `json:"name" db:"name"`
	Role      string          `json:"role" db:"role"`
	LBBackend string          `json:"lb_backend" db:"lb_backend"`
	Config    json.RawMessage `json:"config" db:"config"`
	Status        string          `json:"status" db:"status"`
	StatusMessage *string         `json:"status_message,omitempty" db:"status_message"`
	CreatedAt     time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

const (
	ShardRoleWeb      = "web"
	ShardRoleDatabase = "database"
	ShardRoleDNS      = "dns"
	ShardRoleEmail    = "email"
	ShardRoleValkey   = "valkey"
	ShardRoleStorage     = "storage"
	ShardRoleDBAdmin = "dbadmin"
	ShardRoleLB      = "lb"
	ShardRoleGateway = "gateway"
)

type StorageShardConfig struct {
	S3Enabled        bool `json:"s3_enabled"`
	FilestoreEnabled bool `json:"filestore_enabled"`
}

// DatabaseShardConfig holds replication configuration for a database shard.
type DatabaseShardConfig struct {
	PrimaryNodeID string `json:"primary_node_id"`
}

// GatewayShardConfig holds configuration for a WireGuard gateway shard.
type GatewayShardConfig struct {
	ListenPort   int    `json:"listen_port"`
	PublicKey    string `json:"public_key"`
	ExternalHost string `json:"external_host"`
}
