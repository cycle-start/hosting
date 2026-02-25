package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type RegionService struct {
	db DB
}

func NewRegionService(db DB) *RegionService {
	return &RegionService{db: db}
}

func (s *RegionService) Create(ctx context.Context, region *model.Region) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO regions (id, name, config, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		region.ID, region.Name, region.Config, region.CreatedAt, region.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create region: %w", err)
	}
	return nil
}

func (s *RegionService) GetByID(ctx context.Context, id string) (*model.Region, error) {
	var r model.Region
	err := s.db.QueryRow(ctx,
		"SELECT id, name, config, created_at, updated_at FROM regions WHERE id = $1", id,
	).Scan(&r.ID, &r.Name, &r.Config, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get region %s: %w", id, err)
	}
	return &r, nil
}

func (s *RegionService) List(ctx context.Context, limit int, cursor string) ([]model.Region, bool, error) {
	query := `SELECT id, name, config, created_at, updated_at FROM regions`
	args := []any{}
	argIdx := 1

	if cursor != "" {
		query += fmt.Sprintf(` WHERE id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list regions: %w", err)
	}
	defer rows.Close()

	var regions []model.Region
	for rows.Next() {
		var r model.Region
		if err := rows.Scan(&r.ID, &r.Name, &r.Config, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan region: %w", err)
		}
		regions = append(regions, r)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate regions: %w", err)
	}

	hasMore := len(regions) > limit
	if hasMore {
		regions = regions[:limit]
	}
	return regions, hasMore, nil
}

func (s *RegionService) Update(ctx context.Context, region *model.Region) error {
	_, err := s.db.Exec(ctx,
		`UPDATE regions SET name = $1, config = $2, updated_at = now() WHERE id = $3`,
		region.Name, region.Config, region.ID,
	)
	if err != nil {
		return fmt.Errorf("update region %s: %w", region.ID, err)
	}
	return nil
}

func (s *RegionService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM regions WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete region %s: %w", id, err)
	}
	return nil
}

