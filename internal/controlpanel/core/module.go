package core

import (
	"context"
	"fmt"
)

type ModuleService struct {
	db DB
}

func NewModuleService(db DB) *ModuleService {
	return &ModuleService{db: db}
}

// AllModules is the canonical list of modules in the platform.
var AllModules = []string{"webroots", "dns", "databases", "email", "valkey", "s3", "ssh_keys", "backups", "wireguard"}

// GetEnabledModules returns all modules minus those disabled for the brand.
func (s *ModuleService) GetEnabledModules(ctx context.Context, brandID string) ([]string, error) {
	var disabledModules []string
	err := s.db.QueryRow(ctx,
		`SELECT disabled_modules FROM brand_modules WHERE brand_id = $1`, brandID,
	).Scan(&disabledModules)
	if err != nil {
		// No row means no disabled modules — all enabled.
		return AllModules, nil
	}

	disabled := make(map[string]bool, len(disabledModules))
	for _, m := range disabledModules {
		disabled[m] = true
	}

	var enabled []string
	for _, m := range AllModules {
		if !disabled[m] {
			enabled = append(enabled, m)
		}
	}
	if enabled == nil {
		return nil, fmt.Errorf("all modules disabled for brand %s", brandID)
	}
	return enabled, nil
}
