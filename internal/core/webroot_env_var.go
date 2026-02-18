package core

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/edvin/hosting/internal/crypto"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	temporalclient "go.temporal.io/sdk/client"
)

type WebrootEnvVarService struct {
	db  DB
	tc  temporalclient.Client
	kek []byte // master key (KEK), 32 bytes
}

func NewWebrootEnvVarService(db DB, tc temporalclient.Client, kekHex string) *WebrootEnvVarService {
	var kek []byte
	if kekHex != "" {
		kek, _ = hex.DecodeString(kekHex)
	}
	return &WebrootEnvVarService{db: db, tc: tc, kek: kek}
}

// List returns all env vars for a webroot. Secret values are redacted.
func (s *WebrootEnvVarService) List(ctx context.Context, webrootID string) ([]model.WebrootEnvVar, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, webroot_id, name, value, is_secret, created_at, updated_at
		 FROM webroot_env_vars WHERE webroot_id = $1 ORDER BY name`, webrootID)
	if err != nil {
		return nil, fmt.Errorf("list env vars: %w", err)
	}
	defer rows.Close()

	var vars []model.WebrootEnvVar
	for rows.Next() {
		var v model.WebrootEnvVar
		if err := rows.Scan(&v.ID, &v.WebrootID, &v.Name, &v.Value, &v.IsSecret, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan env var: %w", err)
		}
		if v.IsSecret {
			v.Value = "***"
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

// ListDecrypted returns all env vars for a webroot with secrets decrypted.
// Used internally for convergence/desired state.
func (s *WebrootEnvVarService) ListDecrypted(ctx context.Context, webrootID string) ([]model.WebrootEnvVar, error) {
	var tenantID string
	if err := s.db.QueryRow(ctx, "SELECT tenant_id FROM webroots WHERE id = $1", webrootID).Scan(&tenantID); err != nil {
		return nil, fmt.Errorf("resolve tenant: %w", err)
	}

	rows, err := s.db.Query(ctx,
		`SELECT id, webroot_id, name, value, is_secret, created_at, updated_at
		 FROM webroot_env_vars WHERE webroot_id = $1 ORDER BY name`, webrootID)
	if err != nil {
		return nil, fmt.Errorf("list env vars: %w", err)
	}
	defer rows.Close()

	var vars []model.WebrootEnvVar
	for rows.Next() {
		var v model.WebrootEnvVar
		if err := rows.Scan(&v.ID, &v.WebrootID, &v.Name, &v.Value, &v.IsSecret, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan env var: %w", err)
		}
		if v.IsSecret {
			dek, err := s.getTenantDEK(ctx, tenantID)
			if err != nil {
				return nil, fmt.Errorf("get tenant dek: %w", err)
			}
			plaintext, err := crypto.Decrypt(v.Value, dek)
			if err != nil {
				return nil, fmt.Errorf("decrypt env var %s: %w", v.Name, err)
			}
			v.Value = string(plaintext)
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

// BulkSet replaces all env vars for a webroot. Secrets are encrypted before storage.
func (s *WebrootEnvVarService) BulkSet(ctx context.Context, webrootID string, vars []model.WebrootEnvVar) error {
	var tenantID string
	if err := s.db.QueryRow(ctx, "SELECT tenant_id FROM webroots WHERE id = $1", webrootID).Scan(&tenantID); err != nil {
		return fmt.Errorf("resolve tenant: %w", err)
	}

	// Delete existing vars.
	if _, err := s.db.Exec(ctx, `DELETE FROM webroot_env_vars WHERE webroot_id = $1`, webrootID); err != nil {
		return fmt.Errorf("delete existing env vars: %w", err)
	}

	// Insert new vars.
	now := time.Now()
	for _, v := range vars {
		value := v.Value
		if v.IsSecret {
			dek, err := s.getOrCreateTenantDEK(ctx, tenantID)
			if err != nil {
				return fmt.Errorf("get tenant dek: %w", err)
			}
			encrypted, err := crypto.Encrypt([]byte(value), dek)
			if err != nil {
				return fmt.Errorf("encrypt env var %s: %w", v.Name, err)
			}
			value = encrypted
		}
		_, err := s.db.Exec(ctx,
			`INSERT INTO webroot_env_vars (id, webroot_id, name, value, is_secret, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			platform.NewID(), webrootID, v.Name, value, v.IsSecret, now, now)
		if err != nil {
			return fmt.Errorf("insert env var %s: %w", v.Name, err)
		}
	}

	// Trigger re-convergence via UpdateWebrootWorkflow.
	webroot, err := s.getWebrootByID(ctx, webrootID)
	if err != nil {
		return fmt.Errorf("get webroot for workflow: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateWebrootWorkflow",
		WorkflowID:   workflowID("webroot-env", webroot.Name, webrootID),
		Arg:          webrootID,
	}); err != nil {
		return fmt.Errorf("signal UpdateWebrootWorkflow: %w", err)
	}

	return nil
}

// DeleteByName deletes a single env var by name.
func (s *WebrootEnvVarService) DeleteByName(ctx context.Context, webrootID, name string) error {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM webroot_env_vars WHERE webroot_id = $1 AND name = $2`, webrootID, name)
	if err != nil {
		return fmt.Errorf("delete env var: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("env var %q not found", name)
	}

	// Trigger re-convergence.
	tenantID, err := resolveTenantIDFromWebroot(ctx, s.db, webrootID)
	if err != nil {
		return fmt.Errorf("resolve tenant for env var: %w", err)
	}
	webroot, err := s.getWebrootByID(ctx, webrootID)
	if err != nil {
		return fmt.Errorf("get webroot for workflow: %w", err)
	}
	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateWebrootWorkflow",
		WorkflowID:   workflowID("webroot-env", webroot.Name, webrootID),
		Arg:          webrootID,
	}); err != nil {
		return fmt.Errorf("signal UpdateWebrootWorkflow: %w", err)
	}

	return nil
}

func (s *WebrootEnvVarService) getWebrootByID(ctx context.Context, id string) (*model.Webroot, error) {
	var w model.Webroot
	err := s.db.QueryRow(ctx, `SELECT id, name FROM webroots WHERE id = $1`, id).Scan(&w.ID, &w.Name)
	if err != nil {
		return nil, fmt.Errorf("get webroot %s: %w", id, err)
	}
	return &w, nil
}

// getTenantDEK retrieves and decrypts the tenant's DEK using the KEK.
func (s *WebrootEnvVarService) getTenantDEK(ctx context.Context, tenantID string) ([]byte, error) {
	var encryptedDEK string
	err := s.db.QueryRow(ctx,
		`SELECT encrypted_dek FROM tenant_encryption_keys WHERE tenant_id = $1`, tenantID,
	).Scan(&encryptedDEK)
	if err != nil {
		return nil, fmt.Errorf("get tenant encryption key for %s: %w", tenantID, err)
	}
	dek, err := crypto.Decrypt(encryptedDEK, s.kek)
	if err != nil {
		return nil, fmt.Errorf("decrypt tenant dek: %w", err)
	}
	return dek, nil
}

// getOrCreateTenantDEK retrieves or creates the tenant's DEK.
func (s *WebrootEnvVarService) getOrCreateTenantDEK(ctx context.Context, tenantID string) ([]byte, error) {
	var encryptedDEK string
	err := s.db.QueryRow(ctx,
		`SELECT encrypted_dek FROM tenant_encryption_keys WHERE tenant_id = $1`, tenantID,
	).Scan(&encryptedDEK)
	if err == nil {
		// DEK exists, decrypt and return.
		dek, err := crypto.Decrypt(encryptedDEK, s.kek)
		if err != nil {
			return nil, fmt.Errorf("decrypt tenant dek: %w", err)
		}
		return dek, nil
	}

	// Generate a new DEK.
	dek, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("generate tenant dek: %w", err)
	}

	// Encrypt DEK with KEK.
	encrypted, err := crypto.Encrypt(dek, s.kek)
	if err != nil {
		return nil, fmt.Errorf("encrypt tenant dek: %w", err)
	}

	// Store encrypted DEK.
	_, err = s.db.Exec(ctx,
		`INSERT INTO tenant_encryption_keys (tenant_id, encrypted_dek) VALUES ($1, $2)
		 ON CONFLICT (tenant_id) DO NOTHING`,
		tenantID, encrypted)
	if err != nil {
		return nil, fmt.Errorf("store tenant dek: %w", err)
	}

	return dek, nil
}
