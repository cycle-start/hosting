package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type ClusterService struct {
	db DB
}

func NewClusterService(db DB, tc ...any) *ClusterService {
	return &ClusterService{db: db}
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

func (s *ClusterService) ListByRegion(ctx context.Context, regionID string, limit int, cursor string) ([]model.Cluster, bool, error) {
	query := `SELECT id, region_id, name, config, status, spec, created_at, updated_at FROM clusters WHERE region_id = $1`
	args := []any{regionID}
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
		return nil, false, fmt.Errorf("list clusters for region %s: %w", regionID, err)
	}
	defer rows.Close()

	var clusters []model.Cluster
	for rows.Next() {
		var c model.Cluster
		if err := rows.Scan(&c.ID, &c.RegionID, &c.Name,
			&c.Config, &c.Status, &c.Spec, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan cluster: %w", err)
		}
		clusters = append(clusters, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate clusters: %w", err)
	}

	hasMore := len(clusters) > limit
	if hasMore {
		clusters = clusters[:limit]
	}
	return clusters, hasMore, nil
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

