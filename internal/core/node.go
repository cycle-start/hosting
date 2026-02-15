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

func (s *NodeService) Create(ctx context.Context, node *model.Node) error {
	// Auto-compute shard_index when assigning to a shard.
	if node.ShardID != nil && node.ShardIndex == nil {
		var nextIndex int
		err := s.db.QueryRow(ctx,
			`SELECT COALESCE(MAX(shard_index), 0) + 1 FROM nodes WHERE shard_id = $1`, *node.ShardID,
		).Scan(&nextIndex)
		if err != nil {
			return fmt.Errorf("compute shard_index: %w", err)
		}
		node.ShardIndex = &nextIndex
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO nodes (id, cluster_id, shard_id, shard_index, hostname, ip_address, ip6_address, roles, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		node.ID, node.ClusterID, node.ShardID, node.ShardIndex, node.Hostname, node.IPAddress, node.IP6Address,
		node.Roles, node.Status, node.CreatedAt, node.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}
	return nil
}

func (s *NodeService) GetByID(ctx context.Context, id string) (*model.Node, error) {
	var n model.Node
	err := s.db.QueryRow(ctx,
		`SELECT id, cluster_id, shard_id, shard_index, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.ShardIndex, &n.Hostname, &n.IPAddress, &n.IP6Address,
		&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	return &n, nil
}

func (s *NodeService) ListByCluster(ctx context.Context, clusterID string, params request.ListParams) ([]model.Node, bool, error) {
	query := `SELECT id, cluster_id, shard_id, shard_index, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at FROM nodes WHERE cluster_id = $1 AND status != 'deleted'`
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
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.ShardIndex, &n.Hostname, &n.IPAddress, &n.IP6Address,
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
	return nodes, hasMore, nil
}

func (s *NodeService) ListByShard(ctx context.Context, shardID string, limit int, cursor string) ([]model.Node, bool, error) {
	query := `SELECT id, cluster_id, shard_id, shard_index, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at FROM nodes WHERE shard_id = $1`
	args := []any{shardID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY id`
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
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.ShardIndex, &n.Hostname, &n.IPAddress, &n.IP6Address,
			&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan node: %w", err)
		}
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

func (s *NodeService) Update(ctx context.Context, node *model.Node) error {
	// Auto-compute shard_index when assigning to a shard.
	if node.ShardID != nil && node.ShardIndex == nil {
		var nextIndex int
		err := s.db.QueryRow(ctx,
			`SELECT COALESCE(MAX(shard_index), 0) + 1 FROM nodes WHERE shard_id = $1`, *node.ShardID,
		).Scan(&nextIndex)
		if err != nil {
			return fmt.Errorf("compute shard_index: %w", err)
		}
		node.ShardIndex = &nextIndex
	} else if node.ShardID == nil {
		// Unassigning from shard — clear shard_index.
		node.ShardIndex = nil
	}

	_, err := s.db.Exec(ctx,
		`UPDATE nodes SET shard_id = $1, shard_index = $2, hostname = $3, ip_address = $4, ip6_address = $5, roles = $6, status = $7, updated_at = now()
		 WHERE id = $8`,
		node.ShardID, node.ShardIndex, node.Hostname, node.IPAddress, node.IP6Address, node.Roles, node.Status, node.ID,
	)
	if err != nil {
		return fmt.Errorf("update node %s: %w", node.ID, err)
	}
	return nil
}

func (s *NodeService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM nodes WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete node %s: %w", id, err)
	}
	return nil
}

