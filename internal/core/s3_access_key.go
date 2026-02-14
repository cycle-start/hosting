package core

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type S3AccessKeyService struct {
	db DB
	tc temporalclient.Client
}

func NewS3AccessKeyService(db DB, tc temporalclient.Client) *S3AccessKeyService {
	return &S3AccessKeyService{db: db, tc: tc}
}

const alphanumeric = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func generateRandomString(length int) (string, error) {
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphanumeric))))
		if err != nil {
			return "", fmt.Errorf("generate random string: %w", err)
		}
		result[i] = alphanumeric[n.Int64()]
	}
	return string(result), nil
}

func (s *S3AccessKeyService) Create(ctx context.Context, key *model.S3AccessKey) error {
	accessKeyID, err := generateRandomString(20)
	if err != nil {
		return fmt.Errorf("generate access key id: %w", err)
	}
	secretAccessKey, err := generateRandomString(40)
	if err != nil {
		return fmt.Errorf("generate secret access key: %w", err)
	}

	key.AccessKeyID = accessKeyID
	key.SecretAccessKey = secretAccessKey

	_, err = s.db.Exec(ctx,
		`INSERT INTO s3_access_keys (id, s3_bucket_id, access_key_id, secret_access_key, permissions, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		key.ID, key.S3BucketID, key.AccessKeyID, key.SecretAccessKey,
		key.Permissions, key.Status, key.CreatedAt, key.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert s3 access key: %w", err)
	}

	tenantID, err := resolveTenantIDFromS3AccessKey(ctx, s.db, key.ID)
	if err != nil {
		return fmt.Errorf("create s3 access key: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateS3AccessKeyWorkflow",
		WorkflowID:   workflowID("s3-access-key", key.AccessKeyID, key.ID),
		Arg:          key.ID,
	}); err != nil {
		return fmt.Errorf("start CreateS3AccessKeyWorkflow: %w", err)
	}

	return nil
}

func (s *S3AccessKeyService) GetByID(ctx context.Context, id string) (*model.S3AccessKey, error) {
	var k model.S3AccessKey
	err := s.db.QueryRow(ctx,
		`SELECT id, s3_bucket_id, access_key_id, secret_access_key, permissions, status, status_message, created_at, updated_at
		 FROM s3_access_keys WHERE id = $1`, id,
	).Scan(&k.ID, &k.S3BucketID, &k.AccessKeyID, &k.SecretAccessKey,
		&k.Permissions, &k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get s3 access key %s: %w", id, err)
	}
	return &k, nil
}

func (s *S3AccessKeyService) ListByBucket(ctx context.Context, bucketID string, limit int, cursor string) ([]model.S3AccessKey, bool, error) {
	query := `SELECT id, s3_bucket_id, access_key_id, secret_access_key, permissions, status, status_message, created_at, updated_at FROM s3_access_keys WHERE s3_bucket_id = $1`
	args := []any{bucketID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list s3 access keys for bucket %s: %w", bucketID, err)
	}
	defer rows.Close()

	var keys []model.S3AccessKey
	for rows.Next() {
		var k model.S3AccessKey
		if err := rows.Scan(&k.ID, &k.S3BucketID, &k.AccessKeyID, &k.SecretAccessKey,
			&k.Permissions, &k.Status, &k.StatusMessage, &k.CreatedAt, &k.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan s3 access key: %w", err)
		}
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate s3 access keys: %w", err)
	}

	hasMore := len(keys) > limit
	if hasMore {
		keys = keys[:limit]
	}
	return keys, hasMore, nil
}

func (s *S3AccessKeyService) Delete(ctx context.Context, id string) error {
	var accessKeyID string
	err := s.db.QueryRow(ctx,
		"UPDATE s3_access_keys SET status = $1, updated_at = now() WHERE id = $2 RETURNING access_key_id",
		model.StatusDeleting, id,
	).Scan(&accessKeyID)
	if err != nil {
		return fmt.Errorf("set s3 access key %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromS3AccessKey(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete s3 access key: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteS3AccessKeyWorkflow",
		WorkflowID:   workflowID("s3-access-key", accessKeyID, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start DeleteS3AccessKeyWorkflow: %w", err)
	}

	return nil
}

func (s *S3AccessKeyService) Retry(ctx context.Context, id string) error {
	var status, accessKeyID string
	err := s.db.QueryRow(ctx, "SELECT status, access_key_id FROM s3_access_keys WHERE id = $1", id).Scan(&status, &accessKeyID)
	if err != nil {
		return fmt.Errorf("get s3 access key status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("s3 access key %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE s3_access_keys SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set s3 access key %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromS3AccessKey(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry s3 access key: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateS3AccessKeyWorkflow",
		WorkflowID:   workflowID("s3-access-key", accessKeyID, id),
		Arg:          id,
	})
}
