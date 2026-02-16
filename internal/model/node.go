package model

import (
	"encoding/json"
	"time"
)

// NodeShardAssignment represents a node's assignment to a shard.
type NodeShardAssignment struct {
	ShardID    string `json:"shard_id" db:"shard_id"`
	ShardRole  string `json:"shard_role" db:"shard_role"`
	ShardIndex int    `json:"shard_index" db:"shard_index"`
}

type Node struct {
	ID           string     `json:"id" db:"id"`
	ClusterID    string     `json:"cluster_id" db:"cluster_id"`
	Hostname     string     `json:"hostname" db:"hostname"`
	IPAddress    *string    `json:"ip_address,omitempty" db:"ip_address"`
	IP6Address   *string    `json:"ip6_address,omitempty" db:"ip6_address"`
	Roles        []string   `json:"roles" db:"roles"`
	Status       string     `json:"status" db:"status"`
	LastHealthAt *time.Time `json:"last_health_at,omitempty" db:"last_health_at"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at" db:"updated_at"`

	// Populated by GetByID and ListByCluster — all shard assignments for this node.
	Shards []NodeShardAssignment `json:"shards,omitempty"`

	// Transient fields — populated by ListByShard from the join row.
	// Not stored in the nodes table; used by convergence workflows.
	ShardID    *string `json:"shard_id,omitempty"`
	ShardIndex *int    `json:"shard_index,omitempty"`
}

// NodeHealth represents the health report from a node agent.
type NodeHealth struct {
	NodeID         string          `json:"node_id" db:"node_id"`
	Status         string          `json:"status" db:"status"`
	Checks         json.RawMessage `json:"checks" db:"checks"`
	Reconciliation json.RawMessage `json:"reconciliation,omitempty" db:"reconciliation"`
	ReportedAt     time.Time       `json:"reported_at" db:"reported_at"`
}

// DriftEvent represents a drift detection event from a node agent.
type DriftEvent struct {
	ID        string    `json:"id" db:"id"`
	NodeID    string    `json:"node_id" db:"node_id"`
	Kind      string    `json:"kind" db:"kind"`
	Resource  string    `json:"resource" db:"resource"`
	Action    string    `json:"action" db:"action"`
	Detail    string    `json:"detail,omitempty" db:"detail"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
