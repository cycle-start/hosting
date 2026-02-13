package core

import (
	"context"
	"fmt"
)

// DashboardStats holds aggregate counts from the core database.
type DashboardStats struct {
	Regions          int                `json:"regions"`
	Clusters         int                `json:"clusters"`
	Shards           int                `json:"shards"`
	Nodes            int                `json:"nodes"`
	Tenants          int                `json:"tenants"`
	TenantsActive    int                `json:"tenants_active"`
	TenantsSuspended int                `json:"tenants_suspended"`
	Databases        int                `json:"databases"`
	Zones            int                `json:"zones"`
	ValkeyInstances  int                `json:"valkey_instances"`
	FQDNs            int                `json:"fqdns"`
	TenantsPerShard  []ShardTenantCount `json:"tenants_per_shard"`
	NodesPerCluster  []ClusterNodeCount `json:"nodes_per_cluster"`
	TenantsByStatus  []StatusCount      `json:"tenants_by_status"`
}

// ShardTenantCount holds tenant count per shard.
type ShardTenantCount struct {
	ShardID   string `json:"shard_id"`
	ShardName string `json:"shard_name"`
	Role      string `json:"role"`
	Count     int    `json:"count"`
}

// ClusterNodeCount holds node count per cluster.
type ClusterNodeCount struct {
	ClusterID   string `json:"cluster_id"`
	ClusterName string `json:"cluster_name"`
	Count       int    `json:"count"`
}

// StatusCount holds a count grouped by status.
type StatusCount struct {
	Status string `json:"status"`
	Count  int    `json:"count"`
}

// DashboardService queries aggregate stats from the core DB.
type DashboardService struct {
	db DB
}

// NewDashboardService creates a new DashboardService.
func NewDashboardService(db DB) *DashboardService {
	return &DashboardService{db: db}
}

// Stats returns aggregate counts from the core database using a single
// query with CTEs for efficiency.
func (s *DashboardService) Stats(ctx context.Context) (*DashboardStats, error) {
	const countsQuery = `
		WITH region_count AS (
			SELECT count(*) AS c FROM regions
		), cluster_count AS (
			SELECT count(*) AS c FROM clusters WHERE status != 'deleted'
		), shard_count AS (
			SELECT count(*) AS c FROM shards WHERE status != 'deleted'
		), node_count AS (
			SELECT count(*) AS c FROM nodes WHERE status != 'deleted'
		), tenant_count AS (
			SELECT count(*) AS c FROM tenants WHERE status != 'deleted'
		), tenant_active AS (
			SELECT count(*) AS c FROM tenants WHERE status = 'active'
		), tenant_suspended AS (
			SELECT count(*) AS c FROM tenants WHERE status = 'suspended'
		), database_count AS (
			SELECT count(*) AS c FROM databases WHERE status != 'deleted'
		), zone_count AS (
			SELECT count(*) AS c FROM zones WHERE status != 'deleted'
		), valkey_count AS (
			SELECT count(*) AS c FROM valkey_instances WHERE status != 'deleted'
		), fqdn_count AS (
			SELECT count(*) AS c FROM fqdns WHERE status != 'deleted'
		)
		SELECT
			(SELECT c FROM region_count),
			(SELECT c FROM cluster_count),
			(SELECT c FROM shard_count),
			(SELECT c FROM node_count),
			(SELECT c FROM tenant_count),
			(SELECT c FROM tenant_active),
			(SELECT c FROM tenant_suspended),
			(SELECT c FROM database_count),
			(SELECT c FROM zone_count),
			(SELECT c FROM valkey_count),
			(SELECT c FROM fqdn_count)`

	stats := &DashboardStats{}
	err := s.db.QueryRow(ctx, countsQuery).Scan(
		&stats.Regions,
		&stats.Clusters,
		&stats.Shards,
		&stats.Nodes,
		&stats.Tenants,
		&stats.TenantsActive,
		&stats.TenantsSuspended,
		&stats.Databases,
		&stats.Zones,
		&stats.ValkeyInstances,
		&stats.FQDNs,
	)
	if err != nil {
		return nil, fmt.Errorf("dashboard counts: %w", err)
	}

	// Tenants per shard
	tpsRows, err := s.db.Query(ctx,
		`SELECT s.id, s.name, s.role, count(t.id)
		 FROM shards s LEFT JOIN tenants t ON t.shard_id = s.id AND t.status != 'deleted'
		 WHERE s.status != 'deleted'
		 GROUP BY s.id, s.name, s.role
		 ORDER BY count(t.id) DESC`)
	if err != nil {
		return nil, fmt.Errorf("dashboard tenants per shard: %w", err)
	}
	defer tpsRows.Close()

	for tpsRows.Next() {
		var sc ShardTenantCount
		if err := tpsRows.Scan(&sc.ShardID, &sc.ShardName, &sc.Role, &sc.Count); err != nil {
			return nil, fmt.Errorf("scan shard tenant count: %w", err)
		}
		stats.TenantsPerShard = append(stats.TenantsPerShard, sc)
	}
	if err := tpsRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shard tenant counts: %w", err)
	}

	// Nodes per cluster
	npcRows, err := s.db.Query(ctx,
		`SELECT c.id, c.name, count(n.id)
		 FROM clusters c LEFT JOIN nodes n ON n.cluster_id = c.id AND n.status != 'deleted'
		 WHERE c.status != 'deleted'
		 GROUP BY c.id, c.name
		 ORDER BY count(n.id) DESC`)
	if err != nil {
		return nil, fmt.Errorf("dashboard nodes per cluster: %w", err)
	}
	defer npcRows.Close()

	for npcRows.Next() {
		var nc ClusterNodeCount
		if err := npcRows.Scan(&nc.ClusterID, &nc.ClusterName, &nc.Count); err != nil {
			return nil, fmt.Errorf("scan cluster node count: %w", err)
		}
		stats.NodesPerCluster = append(stats.NodesPerCluster, nc)
	}
	if err := npcRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cluster node counts: %w", err)
	}

	// Tenants by status
	tbsRows, err := s.db.Query(ctx,
		`SELECT status, count(*) FROM tenants WHERE status != 'deleted' GROUP BY status ORDER BY count(*) DESC`)
	if err != nil {
		return nil, fmt.Errorf("dashboard tenants by status: %w", err)
	}
	defer tbsRows.Close()

	for tbsRows.Next() {
		var sc StatusCount
		if err := tbsRows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, fmt.Errorf("scan status count: %w", err)
		}
		stats.TenantsByStatus = append(stats.TenantsByStatus, sc)
	}
	if err := tbsRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate status counts: %w", err)
	}

	return stats, nil
}
