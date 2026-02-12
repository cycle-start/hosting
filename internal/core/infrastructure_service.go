package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type InfrastructureServiceService struct {
	db DB
}

func NewInfrastructureServiceService(db DB) *InfrastructureServiceService {
	return &InfrastructureServiceService{db: db}
}

func (s *InfrastructureServiceService) Create(ctx context.Context, svc *model.InfrastructureService) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO infrastructure_services (id, cluster_id, host_machine_id, service_type, container_id, container_name, image, config, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		svc.ID, svc.ClusterID, svc.HostMachineID, svc.ServiceType, svc.ContainerID, svc.ContainerName, svc.Image, svc.Config, svc.Status, svc.CreatedAt, svc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create infrastructure service: %w", err)
	}
	return nil
}

func (s *InfrastructureServiceService) GetByID(ctx context.Context, id string) (*model.InfrastructureService, error) {
	var svc model.InfrastructureService
	err := s.db.QueryRow(ctx,
		`SELECT id, cluster_id, host_machine_id, service_type, container_id, container_name, image, config, status, created_at, updated_at
		 FROM infrastructure_services WHERE id = $1`, id,
	).Scan(&svc.ID, &svc.ClusterID, &svc.HostMachineID, &svc.ServiceType, &svc.ContainerID, &svc.ContainerName, &svc.Image, &svc.Config, &svc.Status, &svc.CreatedAt, &svc.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get infrastructure service %s: %w", id, err)
	}
	return &svc, nil
}

func (s *InfrastructureServiceService) ListByCluster(ctx context.Context, clusterID string) ([]model.InfrastructureService, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, cluster_id, host_machine_id, service_type, container_id, container_name, image, config, status, created_at, updated_at
		 FROM infrastructure_services WHERE cluster_id = $1 ORDER BY service_type`, clusterID,
	)
	if err != nil {
		return nil, fmt.Errorf("list infrastructure services for cluster %s: %w", clusterID, err)
	}
	defer rows.Close()

	var services []model.InfrastructureService
	for rows.Next() {
		var svc model.InfrastructureService
		if err := rows.Scan(&svc.ID, &svc.ClusterID, &svc.HostMachineID, &svc.ServiceType, &svc.ContainerID, &svc.ContainerName, &svc.Image, &svc.Config, &svc.Status, &svc.CreatedAt, &svc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan infrastructure service: %w", err)
		}
		services = append(services, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate infrastructure services: %w", err)
	}
	return services, nil
}

func (s *InfrastructureServiceService) Update(ctx context.Context, svc *model.InfrastructureService) error {
	_, err := s.db.Exec(ctx,
		`UPDATE infrastructure_services SET container_id = $1, container_name = $2, image = $3, config = $4, status = $5, updated_at = now()
		 WHERE id = $6`,
		svc.ContainerID, svc.ContainerName, svc.Image, svc.Config, svc.Status, svc.ID,
	)
	if err != nil {
		return fmt.Errorf("update infrastructure service %s: %w", svc.ID, err)
	}
	return nil
}

func (s *InfrastructureServiceService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM infrastructure_services WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete infrastructure service %s: %w", id, err)
	}
	return nil
}
