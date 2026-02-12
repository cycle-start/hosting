package model

import "time"

type Node struct {
	ID          string  `json:"id" db:"id"`
	ClusterID   string  `json:"cluster_id" db:"cluster_id"`
	ShardID     *string `json:"shard_id,omitempty" db:"shard_id"`
	Hostname    string  `json:"hostname" db:"hostname"`
	IPAddress   *string `json:"ip_address,omitempty" db:"ip_address"`
	IP6Address  *string `json:"ip6_address,omitempty" db:"ip6_address"`
	Roles       []string  `json:"roles" db:"roles"`
	GRPCAddress string    `json:"grpc_address" db:"grpc_address"`
	Status      string    `json:"status" db:"status"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}
