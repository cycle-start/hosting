package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/model"
)

type BrandService struct {
	db DB
}

func NewBrandService(db DB) *BrandService {
	return &BrandService{db: db}
}

func (s *BrandService) Create(ctx context.Context, brand *model.Brand) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO brands (id, name, base_hostname, primary_ns, secondary_ns, hostmaster_email, mail_hostname, spf_includes, dkim_selector, dkim_public_key, dmarc_policy, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		brand.ID, brand.Name, brand.BaseHostname, brand.PrimaryNS, brand.SecondaryNS,
		brand.HostmasterEmail, brand.MailHostname, brand.SPFIncludes, brand.DKIMSelector,
		brand.DKIMPublicKey, brand.DMARCPolicy, brand.Status, brand.CreatedAt, brand.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert brand: %w", err)
	}
	return nil
}

func (s *BrandService) GetByID(ctx context.Context, id string) (*model.Brand, error) {
	var b model.Brand
	err := s.db.QueryRow(ctx,
		`SELECT id, name, base_hostname, primary_ns, secondary_ns, hostmaster_email, mail_hostname, spf_includes, dkim_selector, dkim_public_key, dmarc_policy, status, created_at, updated_at
		 FROM brands WHERE id = $1`, id,
	).Scan(&b.ID, &b.Name, &b.BaseHostname, &b.PrimaryNS, &b.SecondaryNS,
		&b.HostmasterEmail, &b.MailHostname, &b.SPFIncludes, &b.DKIMSelector,
		&b.DKIMPublicKey, &b.DMARCPolicy, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get brand %s: %w", id, err)
	}
	return &b, nil
}

func (s *BrandService) List(ctx context.Context, params request.ListParams) ([]model.Brand, bool, error) {
	query := `SELECT id, name, base_hostname, primary_ns, secondary_ns, hostmaster_email, mail_hostname, spf_includes, dkim_selector, dkim_public_key, dmarc_policy, status, created_at, updated_at FROM brands WHERE true`
	args := []any{}
	argIdx := 1

	if params.Search != "" {
		query += fmt.Sprintf(` AND (id ILIKE $%d OR name ILIKE $%d)`, argIdx, argIdx)
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
	case "name":
		sortCol = "name"
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
		return nil, false, fmt.Errorf("list brands: %w", err)
	}
	defer rows.Close()

	var brands []model.Brand
	for rows.Next() {
		var b model.Brand
		if err := rows.Scan(&b.ID, &b.Name, &b.BaseHostname, &b.PrimaryNS, &b.SecondaryNS,
			&b.HostmasterEmail, &b.MailHostname, &b.SPFIncludes, &b.DKIMSelector,
			&b.DKIMPublicKey, &b.DMARCPolicy, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan brand: %w", err)
		}
		brands = append(brands, b)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate brands: %w", err)
	}

	hasMore := len(brands) > params.Limit
	if hasMore {
		brands = brands[:params.Limit]
	}
	return brands, hasMore, nil
}

func (s *BrandService) Update(ctx context.Context, brand *model.Brand) error {
	_, err := s.db.Exec(ctx,
		`UPDATE brands SET name = $1, base_hostname = $2, primary_ns = $3, secondary_ns = $4,
		 hostmaster_email = $5, mail_hostname = $6, spf_includes = $7, dkim_selector = $8,
		 dkim_public_key = $9, dmarc_policy = $10, status = $11, updated_at = now()
		 WHERE id = $12`,
		brand.Name, brand.BaseHostname, brand.PrimaryNS, brand.SecondaryNS,
		brand.HostmasterEmail, brand.MailHostname, brand.SPFIncludes, brand.DKIMSelector,
		brand.DKIMPublicKey, brand.DMARCPolicy, brand.Status, brand.ID,
	)
	if err != nil {
		return fmt.Errorf("update brand %s: %w", brand.ID, err)
	}
	return nil
}

func (s *BrandService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"DELETE FROM brands WHERE id = $1", id,
	)
	if err != nil {
		return fmt.Errorf("delete brand %s: %w", id, err)
	}
	return nil
}

func (s *BrandService) ListClusters(ctx context.Context, brandID string) ([]string, error) {
	rows, err := s.db.Query(ctx,
		`SELECT cluster_id FROM brand_clusters WHERE brand_id = $1 ORDER BY cluster_id`, brandID,
	)
	if err != nil {
		return nil, fmt.Errorf("list brand clusters: %w", err)
	}
	defer rows.Close()

	var clusterIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan brand cluster: %w", err)
		}
		clusterIDs = append(clusterIDs, id)
	}
	return clusterIDs, rows.Err()
}

func (s *BrandService) SetClusters(ctx context.Context, brandID string, clusterIDs []string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM brand_clusters WHERE brand_id = $1`, brandID)
	if err != nil {
		return fmt.Errorf("clear brand clusters: %w", err)
	}

	for _, clusterID := range clusterIDs {
		_, err := s.db.Exec(ctx,
			`INSERT INTO brand_clusters (brand_id, cluster_id) VALUES ($1, $2)`,
			brandID, clusterID,
		)
		if err != nil {
			return fmt.Errorf("insert brand cluster %s: %w", clusterID, err)
		}
	}
	return nil
}
