package core

import (
	"context"
	"fmt"
	"net"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

type ClusterLBAddressService struct {
	db DB
}

func NewClusterLBAddressService(db DB) *ClusterLBAddressService {
	return &ClusterLBAddressService{db: db}
}

func (s *ClusterLBAddressService) Create(ctx context.Context, clusterID, address, label string) (*model.ClusterLBAddress, error) {
	ip := net.ParseIP(address)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", address)
	}
	family := 4
	if ip.To4() == nil {
		family = 6
	}
	addr := &model.ClusterLBAddress{
		ID:        platform.NewID(),
		ClusterID: clusterID,
		Address:   address,
		Family:    family,
		Label:     label,
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO cluster_lb_addresses (id, cluster_id, address, family, label)
		 VALUES ($1, $2, $3, $4, $5)`,
		addr.ID, addr.ClusterID, addr.Address, addr.Family, addr.Label)
	if err != nil {
		return nil, fmt.Errorf("create cluster LB address: %w", err)
	}
	return addr, nil
}

func (s *ClusterLBAddressService) GetByID(ctx context.Context, id string) (*model.ClusterLBAddress, error) {
	var addr model.ClusterLBAddress
	err := s.db.QueryRow(ctx,
		`SELECT id, cluster_id, address::text, family, label, created_at
		 FROM cluster_lb_addresses WHERE id = $1`, id).
		Scan(&addr.ID, &addr.ClusterID, &addr.Address, &addr.Family, &addr.Label, &addr.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get cluster LB address: %w", err)
	}
	return &addr, nil
}

func (s *ClusterLBAddressService) ListByCluster(ctx context.Context, clusterID string) ([]model.ClusterLBAddress, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, cluster_id, address::text, family, label, created_at
		 FROM cluster_lb_addresses WHERE cluster_id = $1 ORDER BY family, address`, clusterID)
	if err != nil {
		return nil, fmt.Errorf("list cluster LB addresses: %w", err)
	}
	defer rows.Close()
	var addrs []model.ClusterLBAddress
	for rows.Next() {
		var a model.ClusterLBAddress
		if err := rows.Scan(&a.ID, &a.ClusterID, &a.Address, &a.Family, &a.Label, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan cluster LB address: %w", err)
		}
		addrs = append(addrs, a)
	}
	return addrs, nil
}

func (s *ClusterLBAddressService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM cluster_lb_addresses WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete cluster LB address: %w", err)
	}
	return nil
}
