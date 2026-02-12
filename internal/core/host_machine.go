package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type HostMachineService struct {
	db DB
}

func NewHostMachineService(db DB) *HostMachineService {
	return &HostMachineService{db: db}
}

func (s *HostMachineService) Create(ctx context.Context, h *model.HostMachine) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO host_machines (id, cluster_id, hostname, ip_address, docker_host, ca_cert_pem, client_cert_pem, client_key_pem, capacity, roles, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		h.ID, h.ClusterID, h.Hostname, h.IPAddress, h.DockerHost, h.CACertPEM, h.ClientCertPEM, h.ClientKeyPEM, h.Capacity, h.Roles, h.Status, h.CreatedAt, h.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create host machine: %w", err)
	}
	return nil
}

func (s *HostMachineService) GetByID(ctx context.Context, id string) (*model.HostMachine, error) {
	var h model.HostMachine
	err := s.db.QueryRow(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, docker_host, ca_cert_pem, client_cert_pem, client_key_pem, capacity, roles, status, created_at, updated_at
		 FROM host_machines WHERE id = $1`, id,
	).Scan(&h.ID, &h.ClusterID, &h.Hostname, &h.IPAddress, &h.DockerHost, &h.CACertPEM, &h.ClientCertPEM, &h.ClientKeyPEM, &h.Capacity, &h.Roles, &h.Status, &h.CreatedAt, &h.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get host machine %s: %w", id, err)
	}
	return &h, nil
}

func (s *HostMachineService) ListByCluster(ctx context.Context, clusterID string) ([]model.HostMachine, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, cluster_id, hostname, ip_address::text, docker_host, ca_cert_pem, client_cert_pem, client_key_pem, capacity, roles, status, created_at, updated_at
		 FROM host_machines WHERE cluster_id = $1 ORDER BY hostname`, clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list host machines for cluster %s: %w", clusterID, err)
	}
	defer rows.Close()

	var hosts []model.HostMachine
	for rows.Next() {
		var h model.HostMachine
		if err := rows.Scan(&h.ID, &h.ClusterID, &h.Hostname, &h.IPAddress, &h.DockerHost, &h.CACertPEM, &h.ClientCertPEM, &h.ClientKeyPEM, &h.Capacity, &h.Roles, &h.Status, &h.CreatedAt, &h.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan host machine: %w", err)
		}
		hosts = append(hosts, h)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate host machines: %w", err)
	}
	return hosts, nil
}

func (s *HostMachineService) Update(ctx context.Context, h *model.HostMachine) error {
	_, err := s.db.Exec(ctx,
		`UPDATE host_machines SET hostname = $1, ip_address = $2, docker_host = $3, ca_cert_pem = $4, client_cert_pem = $5, client_key_pem = $6, capacity = $7, roles = $8, status = $9, updated_at = now()
		 WHERE id = $10`,
		h.Hostname, h.IPAddress, h.DockerHost, h.CACertPEM, h.ClientCertPEM, h.ClientKeyPEM, h.Capacity, h.Roles, h.Status, h.ID,
	)
	if err != nil {
		return fmt.Errorf("update host machine %s: %w", h.ID, err)
	}
	return nil
}

func (s *HostMachineService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM host_machines WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete host machine %s: %w", id, err)
	}
	return nil
}
