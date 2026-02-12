package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type NodeDeploymentService struct {
	db DB
}

func NewNodeDeploymentService(db DB) *NodeDeploymentService {
	return &NodeDeploymentService{db: db}
}

func (s *NodeDeploymentService) GetByNodeID(ctx context.Context, nodeID string) (*model.NodeDeployment, error) {
	var d model.NodeDeployment
	err := s.db.QueryRow(ctx,
		`SELECT id, node_id, host_machine_id, profile_id, container_id, container_name, image_digest, env_overrides, status, deployed_at, last_health_at, created_at, updated_at
		 FROM node_deployments WHERE node_id = $1`, nodeID,
	).Scan(&d.ID, &d.NodeID, &d.HostMachineID, &d.ProfileID, &d.ContainerID, &d.ContainerName, &d.ImageDigest, &d.EnvOverrides, &d.Status, &d.DeployedAt, &d.LastHealthAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get deployment for node %s: %w", nodeID, err)
	}
	return &d, nil
}

func (s *NodeDeploymentService) ListByHost(ctx context.Context, hostID string) ([]model.NodeDeployment, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, node_id, host_machine_id, profile_id, container_id, container_name, image_digest, env_overrides, status, deployed_at, last_health_at, created_at, updated_at
		 FROM node_deployments WHERE host_machine_id = $1 ORDER BY created_at`, hostID,
	)
	if err != nil {
		return nil, fmt.Errorf("list deployments for host %s: %w", hostID, err)
	}
	defer rows.Close()

	var deployments []model.NodeDeployment
	for rows.Next() {
		var d model.NodeDeployment
		if err := rows.Scan(&d.ID, &d.NodeID, &d.HostMachineID, &d.ProfileID, &d.ContainerID, &d.ContainerName, &d.ImageDigest, &d.EnvOverrides, &d.Status, &d.DeployedAt, &d.LastHealthAt, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node deployment: %w", err)
		}
		deployments = append(deployments, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node deployments: %w", err)
	}
	return deployments, nil
}
