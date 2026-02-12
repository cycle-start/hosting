package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type NodeProfileService struct {
	db DB
}

func NewNodeProfileService(db DB) *NodeProfileService {
	return &NodeProfileService{db: db}
}

func (s *NodeProfileService) Create(ctx context.Context, p *model.NodeProfile) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO node_profiles (id, name, role, image, env, volumes, ports, resources, health_check, privileged, network_mode, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		p.ID, p.Name, p.Role, p.Image, p.Env, p.Volumes, p.Ports, p.Resources, p.HealthCheck, p.Privileged, p.NetworkMode, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create node profile: %w", err)
	}
	return nil
}

func (s *NodeProfileService) GetByID(ctx context.Context, id string) (*model.NodeProfile, error) {
	var p model.NodeProfile
	err := s.db.QueryRow(ctx,
		`SELECT id, name, role, image, env, volumes, ports, resources, health_check, privileged, network_mode, created_at, updated_at
		 FROM node_profiles WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.Role, &p.Image, &p.Env, &p.Volumes, &p.Ports, &p.Resources, &p.HealthCheck, &p.Privileged, &p.NetworkMode, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node profile %s: %w", id, err)
	}
	return &p, nil
}

func (s *NodeProfileService) List(ctx context.Context) ([]model.NodeProfile, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, role, image, env, volumes, ports, resources, health_check, privileged, network_mode, created_at, updated_at
		 FROM node_profiles ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list node profiles: %w", err)
	}
	defer rows.Close()

	var profiles []model.NodeProfile
	for rows.Next() {
		var p model.NodeProfile
		if err := rows.Scan(&p.ID, &p.Name, &p.Role, &p.Image, &p.Env, &p.Volumes, &p.Ports, &p.Resources, &p.HealthCheck, &p.Privileged, &p.NetworkMode, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan node profile: %w", err)
		}
		profiles = append(profiles, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate node profiles: %w", err)
	}
	return profiles, nil
}

func (s *NodeProfileService) GetByRole(ctx context.Context, role string) (*model.NodeProfile, error) {
	var p model.NodeProfile
	err := s.db.QueryRow(ctx,
		`SELECT id, name, role, image, env, volumes, ports, resources, health_check, privileged, network_mode, created_at, updated_at
		 FROM node_profiles WHERE role = $1 LIMIT 1`, role,
	).Scan(&p.ID, &p.Name, &p.Role, &p.Image, &p.Env, &p.Volumes, &p.Ports, &p.Resources, &p.HealthCheck, &p.Privileged, &p.NetworkMode, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get node profile for role %s: %w", role, err)
	}
	return &p, nil
}

func (s *NodeProfileService) Update(ctx context.Context, p *model.NodeProfile) error {
	_, err := s.db.Exec(ctx,
		`UPDATE node_profiles SET name = $1, role = $2, image = $3, env = $4, volumes = $5, ports = $6, resources = $7, health_check = $8, privileged = $9, network_mode = $10, updated_at = now()
		 WHERE id = $11`,
		p.Name, p.Role, p.Image, p.Env, p.Volumes, p.Ports, p.Resources, p.HealthCheck, p.Privileged, p.NetworkMode, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update node profile %s: %w", p.ID, err)
	}
	return nil
}

func (s *NodeProfileService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM node_profiles WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete node profile %s: %w", id, err)
	}
	return nil
}
