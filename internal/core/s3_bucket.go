package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type S3BucketService struct {
	db DB
	tc temporalclient.Client
}

func NewS3BucketService(db DB, tc temporalclient.Client) *S3BucketService {
	return &S3BucketService{db: db, tc: tc}
}

func (s *S3BucketService) Create(ctx context.Context, bucket *model.S3Bucket) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO s3_buckets (id, tenant_id, name, shard_id, public, quota_bytes, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		bucket.ID, bucket.TenantID, bucket.Name, bucket.ShardID,
		bucket.Public, bucket.QuotaBytes, bucket.Status, bucket.CreatedAt, bucket.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert s3 bucket: %w", err)
	}

	var tenantID string
	if bucket.TenantID != nil {
		tenantID = *bucket.TenantID
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateS3BucketWorkflow",
		WorkflowID:   fmt.Sprintf("s3-bucket-%s", bucket.ID),
		Arg:          bucket.ID,
	}); err != nil {
		return fmt.Errorf("start CreateS3BucketWorkflow: %w", err)
	}

	return nil
}

func (s *S3BucketService) GetByID(ctx context.Context, id string) (*model.S3Bucket, error) {
	var b model.S3Bucket
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, name, shard_id, public, quota_bytes, status, created_at, updated_at
		 FROM s3_buckets WHERE id = $1`, id,
	).Scan(&b.ID, &b.TenantID, &b.Name, &b.ShardID,
		&b.Public, &b.QuotaBytes, &b.Status, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get s3 bucket %s: %w", id, err)
	}
	return &b, nil
}

func (s *S3BucketService) ListByTenant(ctx context.Context, tenantID string, params request.ListParams) ([]model.S3Bucket, bool, error) {
	query := `SELECT id, tenant_id, name, shard_id, public, quota_bytes, status, created_at, updated_at FROM s3_buckets WHERE tenant_id = $1 AND status != 'deleted'`
	args := []any{tenantID}
	argIdx := 2

	if params.Search != "" {
		query += fmt.Sprintf(` AND name ILIKE $%d`, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}
	if params.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, params.Status)
		argIdx++
	}
	if params.Cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, params.Cursor)
		argIdx++
	}

	sortCol := "created_at"
	switch params.Sort {
	case "name":
		sortCol = "name"
	case "status":
		sortCol = "status"
	case "created_at":
		sortCol = "created_at"
	}
	order := "DESC"
	if params.Order == "asc" {
		order = "ASC"
	}
	query += fmt.Sprintf(` ORDER BY %s %s`, sortCol, order)
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, params.Limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list s3 buckets for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var buckets []model.S3Bucket
	for rows.Next() {
		var b model.S3Bucket
		if err := rows.Scan(&b.ID, &b.TenantID, &b.Name, &b.ShardID,
			&b.Public, &b.QuotaBytes, &b.Status, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan s3 bucket: %w", err)
		}
		buckets = append(buckets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate s3 buckets: %w", err)
	}

	hasMore := len(buckets) > params.Limit
	if hasMore {
		buckets = buckets[:params.Limit]
	}
	return buckets, hasMore, nil
}

func (s *S3BucketService) Update(ctx context.Context, id string, public *bool, quotaBytes *int64) error {
	if public != nil {
		_, err := s.db.Exec(ctx,
			"UPDATE s3_buckets SET public = $1, updated_at = now() WHERE id = $2",
			*public, id,
		)
		if err != nil {
			return fmt.Errorf("update s3 bucket %s public: %w", id, err)
		}
	}
	if quotaBytes != nil {
		_, err := s.db.Exec(ctx,
			"UPDATE s3_buckets SET quota_bytes = $1, updated_at = now() WHERE id = $2",
			*quotaBytes, id,
		)
		if err != nil {
			return fmt.Errorf("update s3 bucket %s quota: %w", id, err)
		}
	}

	tenantID, err := resolveTenantIDFromS3Bucket(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("update s3 bucket: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "UpdateS3BucketWorkflow",
		WorkflowID:   fmt.Sprintf("s3-bucket-%s", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start UpdateS3BucketWorkflow: %w", err)
	}

	return nil
}

func (s *S3BucketService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx,
		"UPDATE s3_buckets SET status = $1, updated_at = now() WHERE id = $2",
		model.StatusDeleting, id,
	)
	if err != nil {
		return fmt.Errorf("set s3 bucket %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromS3Bucket(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete s3 bucket: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteS3BucketWorkflow",
		WorkflowID:   fmt.Sprintf("s3-bucket-%s", id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("start DeleteS3BucketWorkflow: %w", err)
	}

	return nil
}
