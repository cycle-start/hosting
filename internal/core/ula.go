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

// FormatDaemonProxyURL formats a proxy_pass URL for nginx, handling both
// IPv4 and IPv6 addresses. IPv6 addresses must be wrapped in square brackets
// per RFC 2732.
func FormatDaemonProxyURL(targetIP string, port int) string {
	if strings.Contains(targetIP, ":") {
		return fmt.Sprintf("http://[%s]:%d", targetIP, port)
	}
	return fmt.Sprintf("http://%s:%d", targetIP, port)
}
