package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
)

type PlatformConfigService struct {
	db DB
}

func NewPlatformConfigService(db DB) *PlatformConfigService {
	return &PlatformConfigService{db: db}
}

func (s *PlatformConfigService) Get(ctx context.Context, key string) (*model.PlatformConfig, error) {
	var cfg model.PlatformConfig
	err := s.db.QueryRow(ctx,
		"SELECT key, value FROM platform_config WHERE key = $1", key,
	).Scan(&cfg.Key, &cfg.Value)
	if err != nil {
		return nil, fmt.Errorf("get platform config %q: %w", key, err)
	}
	return &cfg, nil
}

func (s *PlatformConfigService) GetAll(ctx context.Context) ([]model.PlatformConfig, error) {
	rows, err := s.db.Query(ctx, "SELECT key, value FROM platform_config ORDER BY key")
	if err != nil {
		return nil, fmt.Errorf("list platform config: %w", err)
	}
	defer rows.Close()

	var configs []model.PlatformConfig
	for rows.Next() {
		var cfg model.PlatformConfig
		if err := rows.Scan(&cfg.Key, &cfg.Value); err != nil {
			return nil, fmt.Errorf("scan platform config: %w", err)
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate platform config: %w", err)
	}
	return configs, nil
}

func (s *PlatformConfigService) Set(ctx context.Context, key, value string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO platform_config (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, value,
	)
	if err != nil {
		return fmt.Errorf("set platform config %q: %w", key, err)
	}
	return nil
}
