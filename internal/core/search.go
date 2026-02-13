package core

import (
	"context"
	"fmt"

	"golang.org/x/sync/errgroup"
)

// SearchResult represents a single search result across resource types.
type SearchResult struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Label    string `json:"label"`
	TenantID string `json:"tenant_id,omitempty"`
	Status   string `json:"status"`
}

// SearchService provides cross-resource search.
type SearchService struct {
	db DB
}

// NewSearchService creates a new SearchService.
func NewSearchService(db DB) *SearchService {
	return &SearchService{db: db}
}

// Search runs parallel queries across resource tables and returns matching results.
func (s *SearchService) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 5
	}
	pattern := "%" + query + "%"

	type queryDef struct {
		sql  string
		args []any
	}

	queries := []queryDef{
		{
			sql: `SELECT 'brand', id, name, '', status FROM brands
				WHERE status != 'deleted' AND (id ILIKE $1 OR name ILIKE $1)
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 'tenant', id, id, '', status FROM tenants
				WHERE status != 'deleted' AND id ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 'zone', id, name, tenant_id, status FROM zones
				WHERE status != 'deleted' AND name ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 'webroot', id, name, tenant_id, status FROM webroots
				WHERE status != 'deleted' AND name ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 'fqdn', f.id, f.fqdn, w.tenant_id, f.status
				FROM fqdns f JOIN webroots w ON f.webroot_id = w.id
				WHERE f.status != 'deleted' AND f.fqdn ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 'database', id, name, tenant_id, status FROM databases
				WHERE status != 'deleted' AND name ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 'email_account', e.id, e.address, w.tenant_id, e.status
				FROM email_accounts e JOIN fqdns f ON e.fqdn_id = f.id JOIN webroots w ON f.webroot_id = w.id
				WHERE e.status != 'deleted' AND e.address ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 'valkey_instance', id, name, tenant_id, status FROM valkey_instances
				WHERE status != 'deleted' AND name ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
		{
			sql: `SELECT 's3_bucket', id, name, COALESCE(tenant_id, ''), status FROM s3_buckets
				WHERE status != 'deleted' AND name ILIKE $1
				LIMIT $2`,
			args: []any{pattern, limit},
		},
	}

	results := make([][]SearchResult, len(queries))
	g, ctx := errgroup.WithContext(ctx)

	for i, q := range queries {
		g.Go(func() error {
			rows, err := s.db.Query(ctx, q.sql, q.args...)
			if err != nil {
				return fmt.Errorf("search query %d: %w", i, err)
			}
			defer rows.Close()

			for rows.Next() {
				var r SearchResult
				if err := rows.Scan(&r.Type, &r.ID, &r.Label, &r.TenantID, &r.Status); err != nil {
					return fmt.Errorf("scan search result: %w", err)
				}
				results[i] = append(results[i], r)
			}
			return rows.Err()
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	var all []SearchResult
	for _, batch := range results {
		all = append(all, batch...)
	}
	return all, nil
}
