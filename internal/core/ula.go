package core

import (
	"fmt"
	"hash/fnv"
	"strings"
)

// ComputeClusterHash computes the 16-bit cluster hash used in ULA addressing.
func ComputeClusterHash(clusterID string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(clusterID))
	return h.Sum32() % 0xFFFF
}

// ComputeTenantULA computes the per-tenant per-node ULA IPv6 address.
// Format: fd00:{cluster_hash}:{node_shard_index}::{tenant_uid_hex}
// The cluster hash is the FNV-32a hash of the cluster ID modulo 0xFFFF.
func ComputeTenantULA(clusterID string, nodeShardIndex int, tenantUID int) string {
	clusterHash := ComputeClusterHash(clusterID)
	return fmt.Sprintf("fd00:%x:%x::%x", clusterHash, nodeShardIndex, tenantUID)
}

// Transit address offset constants. Each role gets a 256-slot range in the
// transit address space fd00:{hash}:0::{transit_index} so that web, DB, and
// Valkey nodes within the same cluster never collide.
const (
	TransitOffsetWeb      = 0   // 0-255
	TransitOffsetDatabase = 256 // 256-511
	TransitOffsetValkey   = 512 // 512-767
)

// TransitIndex computes the transit address index for a node given its shard
// role and shard-local index. This prevents collisions when web, DB, and Valkey
// nodes share the same cluster hash in the transit address space.
func TransitIndex(shardRole string, shardIndex int) int {
	switch shardRole {
	case "database":
		return TransitOffsetDatabase + shardIndex
	case "valkey":
		return TransitOffsetValkey + shardIndex
	default:
		return TransitOffsetWeb + shardIndex
	}
}

// FormatDaemonProxyURL formats a proxy_pass URL for nginx, handling both
// IPv4 and IPv6 addresses. IPv6 addresses must be wrapped in square brackets
// per RFC 2732.
func FormatDaemonProxyURL(targetIP string, port int) string {
	if strings.Contains(targetIP, ":") {
		return fmt.Sprintf("http://[%s]:%d", targetIP, port)
	}
	return fmt.Sprintf("http://%s:%d", targetIP, port)
}
