package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type NodeService struct {
	db DB
}

func NewNodeService(db DB, tc ...any) *NodeService {
	return &NodeService{db: db}
}

func (s *NodeService) Create(ctx context.Context, node *model.Node) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO nodes (id, cluster_id, shard_id, hostname, ip_address, ip6_address, roles, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		node.ID, node.ClusterID, node.ShardID, node.Hostname, node.IPAddress, node.IP6Address,
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
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.Hostname, &n.IPAddress, &n.IP6Address,
		&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	return &n, nil
}

func (s *NodeService) ListByCluster(ctx context.Context, clusterID string) ([]model.Node, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE cluster_id = $1 ORDER BY hostname`, clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nodes for cluster %s: %w", clusterID, err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.Hostname, &n.IPAddress, &n.IP6Address,
			&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	return nodes, nil
}

func (s *NodeService) ListByShard(ctx context.Context, shardID string) ([]model.Node, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, status, created_at, updated_at
		 FROM nodes WHERE shard_id = $1 ORDER BY hostname`, shardID,
	)
	if err != nil {
		return nil, fmt.Errorf("list nodes for shard %s: %w", shardID, err)
	}
	defer rows.Close()

	var nodes []model.Node
	for rows.Next() {
		var n model.Node
		if err := rows.Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.Hostname, &n.IPAddress, &n.IP6Address,
			&n.Roles, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node: %w", err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate nodes: %w", err)
	}
	return nodes, nil
}

func (s *NodeService) Update(ctx context.Context, node *model.Node) error {
	_, err := s.db.Exec(ctx,
		`UPDATE nodes SET shard_id = $1, hostname = $2, ip_address = $3, ip6_address = $4, roles = $5, status = $6, updated_at = now()
		 WHERE id = $7`,
		node.ShardID, node.Hostname, node.IPAddress, node.IP6Address, node.Roles, node.Status, node.ID,
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

