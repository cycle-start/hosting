package core

import (
	"context"

	"github.com/edvin/hosting/internal/controlpanel/model"
)

type PartnerService struct {
	db DB
}

func NewPartnerService(db DB) *PartnerService {
	return &PartnerService{db: db}
}

// GetByHostname looks up a partner by its configured hostname.
func (s *PartnerService) GetByHostname(ctx context.Context, hostname string) (*model.Partner, error) {
	var p model.Partner
	err := s.db.QueryRow(ctx,
		`SELECT id, brand_id, name, hostname, primary_color, status, created_at, updated_at
		 FROM partners WHERE hostname = $1 AND status = 'active'`, hostname,
	).Scan(&p.ID, &p.BrandID, &p.Name, &p.Hostname, &p.PrimaryColor, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
