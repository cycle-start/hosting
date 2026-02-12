package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type NodeService struct {
	db DB
	tc temporalclient.Client
}

func NewNodeService(db DB, tc ...temporalclient.Client) *NodeService {
	s := &NodeService{db: db}
	if len(tc) > 0 {
		s.tc = tc[0]
	}
	return s
}

func (s *NodeService) Create(ctx context.Context, node *model.Node) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO nodes (id, cluster_id, shard_id, hostname, ip_address, ip6_address, roles, grpc_address, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		node.ID, node.ClusterID, node.ShardID, node.Hostname, node.IPAddress, node.IP6Address,
		node.Roles, node.GRPCAddress, node.Status, node.CreatedAt, node.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create node: %w", err)
	}
	return nil
}

func (s *NodeService) GetByID(ctx context.Context, id string) (*model.Node, error) {
	var n model.Node
	err := s.db.QueryRow(ctx,
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, grpc_address, status, created_at, updated_at
		 FROM nodes WHERE id = $1`, id,
	).Scan(&n.ID, &n.ClusterID, &n.ShardID, &n.Hostname, &n.IPAddress, &n.IP6Address,
		&n.Roles, &n.GRPCAddress, &n.Status, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node %s: %w", id, err)
	}
	return &n, nil
}

func (s *NodeService) ListByCluster(ctx context.Context, clusterID string) ([]model.Node, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, grpc_address, status, created_at, updated_at
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
			&n.Roles, &n.GRPCAddress, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
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
		`SELECT id, cluster_id, shard_id, hostname, ip_address::text, ip6_address::text, roles, grpc_address, status, created_at, updated_at
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
			&n.Roles, &n.GRPCAddress, &n.Status, &n.CreatedAt, &n.UpdatedAt); err != nil {
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
		`UPDATE nodes SET shard_id = $1, hostname = $2, ip_address = $3, ip6_address = $4, roles = $5, grpc_address = $6, status = $7, updated_at = now()
		 WHERE id = $8`,
		node.ShardID, node.Hostname, node.IPAddress, node.IP6Address, node.Roles, node.GRPCAddress, node.Status, node.ID,
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

func (s *NodeService) Provision(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE nodes SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusPending, id,
	)
	if err != nil {
		return fmt.Errorf("set node %s status to pending: %w", id, err)
	}

	workflowID := fmt.Sprintf("provision-node-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "ProvisionNodeWorkflow", id)
	if err != nil {
		return fmt.Errorf("start ProvisionNodeWorkflow: %w", err)
	}

	return nil
}

func (s *NodeService) Decommission(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE nodes SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set node %s status to deleting: %w", id, err)
	}

	workflowID := fmt.Sprintf("decommission-node-%s", id)
	_, err = s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DecommissionNodeWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DecommissionNodeWorkflow: %w", err)
	}

	return nil
}
