package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type ClusterService struct {
	db DB
	tc temporalclient.Client
}

func NewClusterService(db DB, tc ...temporalclient.Client) *ClusterService {
	s := &ClusterService{db: db}
	if len(tc) > 0 {
		s.tc = tc[0]
	}
	return s
}

func (s *ClusterService) Create(ctx context.Context, cluster *model.Cluster) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO clusters (id, region_id, name, config, status, spec, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		cluster.ID, cluster.RegionID, cluster.Name,
		cluster.Config, cluster.Status, cluster.Spec, cluster.CreatedAt, cluster.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create cluster: %w", err)
	}
	return nil
}

func (s *ClusterService) GetByID(ctx context.Context, id string) (*model.Cluster, error) {
	var c model.Cluster
	err := s.db.QueryRow(ctx,
		`SELECT id, region_id, name, config, status, spec, created_at, updated_at
		 FROM clusters WHERE id = $1`, id,
	).Scan(&c.ID, &c.RegionID, &c.Name,
		&c.Config, &c.Status, &c.Spec, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get cluster %s: %w", id, err)
	}
	return &c, nil
}

func (s *ClusterService) ListByRegion(ctx context.Context, regionID string) ([]model.Cluster, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, region_id, name, config, status, spec, created_at, updated_at
		 FROM clusters WHERE region_id = $1 ORDER BY name`, regionID,
	)
	if err != nil {
		return nil, fmt.Errorf("list clusters for region %s: %w", regionID, err)
	}
	defer rows.Close()

	var clusters []model.Cluster
	for rows.Next() {
		var c model.Cluster
		if err := rows.Scan(&c.ID, &c.RegionID, &c.Name,
			&c.Config, &c.Status, &c.Spec, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan cluster: %w", err)
		}
		clusters = append(clusters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate clusters: %w", err)
	}
	return clusters, nil
}

func (s *ClusterService) Update(ctx context.Context, cluster *model.Cluster) error {
	_, err := s.db.Exec(ctx,
		`UPDATE clusters SET name = $1, config = $2, status = $3, spec = $4, updated_at = now()
		 WHERE id = $5`,
		cluster.Name, cluster.Config, cluster.Status, cluster.Spec, cluster.ID,
	)
	if err != nil {
		return fmt.Errorf("update cluster %s: %w", cluster.ID, err)
	}
	return nil
}

func (s *ClusterService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM clusters WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete cluster %s: %w", id, err)
	}
	return nil
}

func (s *ClusterService) Provision(ctx context.Context, id string) error {
	workflowID := fmt.Sprintf("cluster-provision-%s", id)
	_, err := s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "ProvisionClusterWorkflow", id)
	if err != nil {
		return fmt.Errorf("start ProvisionClusterWorkflow: %w", err)
	}
	return nil
}

func (s *ClusterService) Decommission(ctx context.Context, id string) error {
	workflowID := fmt.Sprintf("cluster-decommission-%s", id)
	_, err := s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "hosting-tasks",
	}, "DecommissionClusterWorkflow", id)
	if err != nil {
		return fmt.Errorf("start DecommissionClusterWorkflow: %w", err)
	}
	return nil
}
