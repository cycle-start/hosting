package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/controlpanel/model"
)

type ProductService struct {
	db DB
}

func NewProductService(db DB) *ProductService {
	return &ProductService{db: db}
}

// GetByID returns a single product by its ID.
func (s *ProductService) GetByID(ctx context.Context, id string) (*model.Product, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, brand_id, name, description, modules, status, created_at, updated_at
		 FROM products
		 WHERE id = $1`, id)

	var p model.Product
	if err := row.Scan(&p.ID, &p.BrandID, &p.Name, &p.Description, &p.Modules, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, fmt.Errorf("get product %s: %w", id, err)
	}
	return &p, nil
}

// ListByBrand returns all active products for a brand.
func (s *ProductService) ListByBrand(ctx context.Context, brandID string) ([]model.Product, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, brand_id, name, description, modules, status, created_at, updated_at
		 FROM products
		 WHERE brand_id = $1 AND status = 'active'
		 ORDER BY name`, brandID)
	if err != nil {
		return nil, fmt.Errorf("list products for brand %s: %w", brandID, err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.ID, &p.BrandID, &p.Name, &p.Description, &p.Modules, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}
