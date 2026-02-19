package core

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

// APIKeyService manages API key operations against the core database.
type APIKeyService struct {
	db DB
}

// NewAPIKeyService creates a new APIKeyService.
func NewAPIKeyService(db DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// Create generates a new API key, stores the hash, and returns the model along
// with the raw key string. The raw key must be shown to the user exactly once.
func (s *APIKeyService) Create(ctx context.Context, name string, scopes, brands []string) (*model.APIKey, string, error) {
	// Generate a random 32-byte key.
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return nil, "", fmt.Errorf("generate api key: %w", err)
	}
	rawKey := "hst_" + hex.EncodeToString(rawBytes) // 68 chars total

	return s.createWithKey(ctx, name, rawKey, scopes, brands)
}

// CreateWithRawKey stores an API key with a caller-provided raw key value.
// Used for well-known dev/test keys where the raw value must be deterministic.
func (s *APIKeyService) CreateWithRawKey(ctx context.Context, name, rawKey string, scopes, brands []string) (*model.APIKey, error) {
	key, _, err := s.createWithKey(ctx, name, rawKey, scopes, brands)
	if err != nil {
		return nil, err
	}
	return key, nil
}

func (s *APIKeyService) createWithKey(ctx context.Context, name, rawKey string, scopes, brands []string) (*model.APIKey, string, error) {
	id := platform.NewID()

	hash := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(hash[:])
	keyPrefix := rawKey[:12] // "hst_" + first 8 hex chars

	if scopes == nil {
		scopes = []string{"*:*"}
	}
	if brands == nil {
		brands = []string{"*"}
	}

	_, err := s.db.Exec(ctx,
		`INSERT INTO api_keys (id, name, key_hash, key_prefix, scopes, brands, created_at) VALUES ($1, $2, $3, $4, $5, $6, now())`,
		id, name, keyHash, keyPrefix, scopes, brands,
	)
	if err != nil {
		return nil, "", fmt.Errorf("insert api key: %w", err)
	}

	key := &model.APIKey{
		ID:        id,
		Name:      name,
		KeyPrefix: keyPrefix,
		Scopes:    scopes,
		Brands:    brands,
	}
	// Fetch the server-generated created_at.
	err = s.db.QueryRow(ctx, "SELECT created_at FROM api_keys WHERE id = $1", id).Scan(&key.CreatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("get api key created_at: %w", err)
	}

	return key, rawKey, nil
}

// GetByID retrieves an API key by its ID.
func (s *APIKeyService) GetByID(ctx context.Context, id string) (*model.APIKey, error) {
	var k model.APIKey
	err := s.db.QueryRow(ctx,
		`SELECT id, name, key_prefix, scopes, brands, created_at, revoked_at FROM api_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.Brands, &k.CreatedAt, &k.RevokedAt)
	if err != nil {
		return nil, fmt.Errorf("get api key %s: %w", id, err)
	}
	return &k, nil
}

// List retrieves API keys with cursor-based pagination.
func (s *APIKeyService) List(ctx context.Context, limit int, cursor string) ([]model.APIKey, bool, error) {
	query := `SELECT id, name, key_prefix, scopes, brands, created_at, revoked_at FROM api_keys WHERE 1=1`
	args := []any{}
	argIdx := 1

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY created_at DESC`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	var keys []model.APIKey
	for rows.Next() {
		var k model.APIKey
		if err := rows.Scan(&k.ID, &k.Name, &k.KeyPrefix, &k.Scopes, &k.Brands, &k.CreatedAt, &k.RevokedAt); err != nil {
			return nil, false, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate api keys: %w", err)
	}

	hasMore := len(keys) > limit
	if hasMore {
		keys = keys[:limit]
	}
	return keys, hasMore, nil
}

// Update modifies the name, scopes, and brands of an API key.
func (s *APIKeyService) Update(ctx context.Context, id, name string, scopes, brands []string) (*model.APIKey, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE api_keys SET name = $1, scopes = $2, brands = $3 WHERE id = $4 AND revoked_at IS NULL`,
		name, scopes, brands, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update api key %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return nil, fmt.Errorf("api key %s not found or already revoked", id)
	}
	return s.GetByID(ctx, id)
}

// Revoke soft-deletes an API key by setting revoked_at.
func (s *APIKeyService) Revoke(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx,
		"UPDATE api_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL", id,
	)
	if err != nil {
		return fmt.Errorf("revoke api key %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("api key %s not found or already revoked", id)
	}
	return nil
}
