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

	IncidentsOpen      int           `json:"incidents_open"`
	IncidentsCritical  int           `json:"incidents_critical"`
	IncidentsEscalated int           `json:"incidents_escalated"`
	IncidentsByStatus  []StatusCount `json:"incidents_by_status"`
	CapabilityGapsOpen int           `json:"capability_gaps_open"`
	MTTRMinutes        *float64      `json:"mttr_minutes"`
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
			SELECT count(*) AS c FROM clusters
		), shard_count AS (
			SELECT count(*) AS c FROM shards
		), node_count AS (
			SELECT count(*) AS c FROM nodes
		), tenant_count AS (
			SELECT count(*) AS c FROM tenants
		), tenant_active AS (
			SELECT count(*) AS c FROM tenants WHERE status = 'active'
		), tenant_suspended AS (
			SELECT count(*) AS c FROM tenants WHERE status = 'suspended'
		), database_count AS (
			SELECT count(*) AS c FROM databases
		), zone_count AS (
			SELECT count(*) AS c FROM zones
		), valkey_count AS (
			SELECT count(*) AS c FROM valkey_instances
		), fqdn_count AS (
			SELECT count(*) AS c FROM fqdns
		), incident_open AS (
			SELECT count(*) AS c FROM incidents WHERE status NOT IN ('resolved', 'cancelled')
		), incident_critical AS (
			SELECT count(*) AS c FROM incidents WHERE severity = 'critical' AND status NOT IN ('resolved', 'cancelled')
		), incident_escalated AS (
			SELECT count(*) AS c FROM incidents WHERE status = 'escalated'
		), capability_gap_open AS (
			SELECT count(*) AS c FROM capability_gaps WHERE status = 'open'
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
			(SELECT c FROM fqdn_count),
			(SELECT c FROM incident_open),
			(SELECT c FROM incident_critical),
			(SELECT c FROM incident_escalated),
			(SELECT c FROM capability_gap_open)`

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
		&stats.IncidentsOpen,
		&stats.IncidentsCritical,
		&stats.IncidentsEscalated,
		&stats.CapabilityGapsOpen,
	)
	if err != nil {
		return nil, fmt.Errorf("dashboard counts: %w", err)
	}

	// Tenants per shard
	tpsRows, err := s.db.Query(ctx,
		`SELECT s.id, s.name, s.role, count(t.id)
		 FROM shards s LEFT JOIN tenants t ON t.shard_id = s.id
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
		 FROM clusters c LEFT JOIN nodes n ON n.cluster_id = c.id
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
		`SELECT status, count(*) FROM tenants GROUP BY status ORDER BY count(*) DESC`)
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

	// Incidents by status
	ibsRows, err := s.db.Query(ctx,
		`SELECT status, count(*) FROM incidents GROUP BY status ORDER BY count(*) DESC`)
	if err != nil {
		return nil, fmt.Errorf("dashboard incidents by status: %w", err)
	}
	defer ibsRows.Close()

	for ibsRows.Next() {
		var sc StatusCount
		if err := ibsRows.Scan(&sc.Status, &sc.Count); err != nil {
			return nil, fmt.Errorf("scan incident status count: %w", err)
		}
		stats.IncidentsByStatus = append(stats.IncidentsByStatus, sc)
	}
	if err := ibsRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate incident status counts: %w", err)
	}

	// MTTR — average time to resolve for incidents resolved in last 30 days
	var mttr *float64
	err = s.db.QueryRow(ctx,
		`SELECT EXTRACT(EPOCH FROM avg(resolved_at - detected_at)) / 60
		 FROM incidents
		 WHERE status = 'resolved' AND resolved_at > now() - interval '30 days'`).Scan(&mttr)
	if err == nil {
		stats.MTTRMinutes = mttr
	}

	return stats, nil
}
