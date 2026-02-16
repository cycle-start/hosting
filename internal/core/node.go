package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/model"
)

type NodeService struct {
	db DB
}

func NewNodeService(db DB, tc ...any) *NodeService {
	return &NodeService{db: db}
}

func (s *NodeService) Create(ctx context.Context, node *model.Node, shardIDs []string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO nodes (id, cluster_id, hostname, ip_address, ip6_address, roles, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		node.ID, node.ClusterID, node.Hostname, node.IPAddress, node.IP6Address,
		node.Roles, node.Status, node.CreatedAt, node.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}

	if err := s.assignShards(ctx, node.ID, shardIDs); err != nil {
		return err
	}

	// Populate the Shards field on the returned node.
	return s.loadShardAssignments(ctx, node)
}

func (s *NodeService) GetByID(ctx context.Context, id string) (*model.Node, error) {
	var n model.Node
	err := s.db.QueryRow(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address,
		&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}

	if err := s.loadShardAssignments(ctx, &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (s *NodeService) ListByCluster(ctx context.Context, clusterID string, params request.ListParams) ([]model.Node, bool, error) {
	query := `SELECT id, cluster_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at FROM nodes WHERE cluster_id = $1`
	args := []any{clusterID}
	argIdx := 2

	if params.Search != "" {
		query += fmt.Sprintf(` AND hostname ILIKE $%d`, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}
	if params.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}

	sortCol := "created_at"
	switch params.Sort {
	case "hostname":
		sortCol = "hostname"
	case "status":
		sortCol = "status"
	case "created_at":
		sortCol = "created_at"
	}
	order := "DESC"
	if params.Order == "asc" {
		order = "ASC"
	}
	query += fmt.Sprintf(` ORDER BY %s %s`, sortCol, order)
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, params.Limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list nodes for cluster %s: %w", clusterID, err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address,
			&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate nodes: %w", err)
	}

	hasMore := len(nodes) > params.Limit
	if hasMore {
		nodes = nodes[:params.Limit]
	}

	// Batch-load shard assignments for all returned nodes.
	if err := s.batchLoadShardAssignments(ctx, nodes); err != nil {
		return nil, false, err
	}

	return nodes, hasMore, nil
}

func (s *NodeService) ListByShard(ctx context.Context, shardID string, limit int, cursor string) ([]model.Node, bool, error) {
	query := `SELECT n.id, n.cluster_id, n.hostname, n.ip_address::text, n.ip6_address::text, n.roles, n.status, n.created_at, n.updated_at,
	                 nsa.shard_id, nsa.shard_index
	          FROM nodes n
	          JOIN node_shard_assignments nsa ON n.id = nsa.node_id
	          WHERE nsa.shard_id = $1`
	args := []any{shardID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND n.id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY n.id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list nodes for shard %s: %w", shardID, err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		var joinShardID string
		var joinShardIndex int
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.Hostname, &n.IPAddress, &n.IP6Address,
			&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt,
			&joinShardID, &joinShardIndex); err != nil {
			return nil, false, fmt.Errorf("scan node: %w", err)
		}
		// Set transient fields for convergence workflow compatibility.
		n.ShardID = &joinShardID
		n.ShardIndex = &joinShardIndex
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate nodes: %w", err)
	}

	hasMore := len(nodes) > limit
	if hasMore {
		nodes = nodes[:limit]
	}
	return nodes, hasMore, nil
}

func (s *NodeService) Update(ctx context.Context, node *model.Node, shardIDs []string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE nodes SET hostname = $1, ip_address = $2, ip6_address = $3, roles = $4, status = $5, updated_at = now()
		 WHERE id = $6`,
		node.Hostname, node.IPAddress, node.IP6Address, node.Roles, node.Status, node.ID,
	)
	if err != nil {
		return fmt.Errorf("update node %s: %w", node.ID, err)
	}

	if shardIDs != nil {
		// Remove all existing assignments and re-assign.
		_, err = s.db.Exec(ctx, `DELETE FROM node_shard_assignments WHERE node_id = $1`, node.ID)
		if err != nil {
			return fmt.Errorf("clear shard assignments for node %s: %w", node.ID, err)
		}
		if err := s.assignShards(ctx, node.ID, shardIDs); err != nil {
			return err
		}
	}

	// Reload shard assignments on the node.
	return s.loadShardAssignments(ctx, node)
}

func (s *NodeService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM nodes WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete node %s: %w", id, err)
	}
	return nil
}

// assignShards inserts shard assignments for a node, auto-computing shard_index per shard.
func (s *NodeService) assignShards(ctx context.Context, nodeID string, shardIDs []string) error {
	for _, shardID := range shardIDs {
		var nextIndex int
		err := s.db.QueryRow(ctx,
			`SELECT COALESCE(MAX(shard_index), 0) + 1 FROM node_shard_assignments WHERE shard_id = $1`, shardID,
		).Scan(&nextIndex)
		if err != nil {
			return fmt.Errorf("compute shard_index for shard %s: %w", shardID, err)
		}

		_, err = s.db.Exec(ctx,
			`INSERT INTO node_shard_assignments (node_id, shard_id, shard_index) VALUES ($1, $2, $3)`,
			nodeID, shardID, nextIndex,
		)
		if err != nil {
			return fmt.Errorf("assign node %s to shard %s: %w", nodeID, shardID, err)
		}
	}
	return nil
}

// loadShardAssignments populates the Shards field on a single node.
func (s *NodeService) loadShardAssignments(ctx context.Context, node *model.Node) error {
	rows, err := s.db.Query(ctx,
		`SELECT nsa.shard_id, s.role, nsa.shard_index
		 FROM node_shard_assignments nsa
		 JOIN shards s ON nsa.shard_id = s.id
		 WHERE nsa.node_id = $1
		 ORDER BY s.role`, node.ID,
	)
	if err != nil {
		return fmt.Errorf("load shard assignments for node %s: %w", node.ID, err)
	}
	defer rows.Close()

	node.Shards = nil
	for rows.Next() {
		var a model.NodeShardAssignment
		if err := rows.Scan(&a.ShardID, &a.ShardRole, &a.ShardIndex); err != nil {
			return fmt.Errorf("scan shard assignment: %w", err)
		}
		node.Shards = append(node.Shards, a)
	}
	return rows.Err()
}

// batchLoadShardAssignments populates the Shards field on multiple nodes.
func (s *NodeService) batchLoadShardAssignments(ctx context.Context, nodes []model.Node) error {
	if len(nodes) == 0 {
		return nil
	}

	nodeIDs := make([]string, len(nodes))
	for i, n := range nodes {
		nodeIDs[i] = n.ID
	}

	rows, err := s.db.Query(ctx,
		`SELECT nsa.node_id, nsa.shard_id, s.role, nsa.shard_index
		 FROM node_shard_assignments nsa
		 JOIN shards s ON nsa.shard_id = s.id
		 WHERE nsa.node_id = ANY($1)
		 ORDER BY nsa.node_id, s.role`, nodeIDs,
	)
	if err != nil {
		return fmt.Errorf("batch load shard assignments: %w", err)
	}
	defer rows.Close()

	assignmentsByNode := make(map[string][]model.NodeShardAssignment)
	for rows.Next() {
		var nodeID string
		var a model.NodeShardAssignment
		if err := rows.Scan(&nodeID, &a.ShardID, &a.ShardRole, &a.ShardIndex); err != nil {
			return fmt.Errorf("scan shard assignment: %w", err)
		}
		assignmentsByNode[nodeID] = append(assignmentsByNode[nodeID], a)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate shard assignments: %w", err)
	}

	for i := range nodes {
		nodes[i].Shards = assignmentsByNode[nodes[i].ID]
	}
	return nil
}
