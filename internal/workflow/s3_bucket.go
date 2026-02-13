package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateS3BucketWorkflow provisions a new S3 bucket via the node agent.
func CreateS3BucketWorkflow(ctx workflow.Context, bucketID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_buckets",
		ID:     bucketID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the bucket.
	var bucket model.S3Bucket
	err = workflow.ExecuteActivity(ctx, "GetS3BucketByID", bucketID).Get(ctx, &bucket)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return err
	}

	if bucket.ShardID == nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return fmt.Errorf("s3 bucket %s has no shard assigned", bucketID)
	}

	// Look up the tenant for the tenant ID used by RGW.
	var tenantID string
	if bucket.TenantID != nil {
		tenantID = *bucket.TenantID
	}
	if tenantID == "" {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return fmt.Errorf("s3 bucket %s has no tenant assigned", bucketID)
	}

	// Look up nodes in the S3 shard.
	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *bucket.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return err
	}

	if len(nodes) == 0 {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return fmt.Errorf("no nodes found in S3 shard %s", *bucket.ShardID)
	}

	// RGW is cluster-wide, so we only need to execute on the first node.
	internalName := tenantID + "--" + bucket.Name
	nodeCtx := nodeActivityCtx(ctx, nodes[0].ID)

	err = workflow.ExecuteActivity(nodeCtx, "CreateS3Bucket", activity.CreateS3BucketParams{
		TenantID:   tenantID,
		Name:       internalName,
		QuotaBytes: bucket.QuotaBytes,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return err
	}

	// If public, set bucket policy.
	if bucket.Public {
		err = workflow.ExecuteActivity(nodeCtx, "UpdateS3BucketPolicy", activity.UpdateS3BucketPolicyParams{
			TenantID: tenantID,
			Name:     internalName,
			Public:   true,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "s3_buckets", bucketID)
			return err
		}
	}

	// Set status to active.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_buckets",
		ID:     bucketID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Trigger service hostname update for the tenant.
	_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_buckets",
		ID:     bucketID,
		Status: model.StatusActive,
	}).Get(ctx, nil)

	return nil
}

// UpdateS3BucketWorkflow updates the policy/quota of an S3 bucket.
func UpdateS3BucketWorkflow(ctx workflow.Context, bucketID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Look up the bucket.
	var bucket model.S3Bucket
	err := workflow.ExecuteActivity(ctx, "GetS3BucketByID", bucketID).Get(ctx, &bucket)
	if err != nil {
		return err
	}

	if bucket.ShardID == nil {
		return fmt.Errorf("s3 bucket %s has no shard assigned", bucketID)
	}

	var tenantID string
	if bucket.TenantID != nil {
		tenantID = *bucket.TenantID
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *bucket.ShardID).Get(ctx, &nodes)
	if err != nil {
		return err
	}

	if len(nodes) == 0 {
		return fmt.Errorf("no nodes found in S3 shard %s", *bucket.ShardID)
	}

	internalName := tenantID + "--" + bucket.Name
	nodeCtx := nodeActivityCtx(ctx, nodes[0].ID)

	err = workflow.ExecuteActivity(nodeCtx, "UpdateS3BucketPolicy", activity.UpdateS3BucketPolicyParams{
		TenantID: tenantID,
		Name:     internalName,
		Public:   bucket.Public,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}

// DeleteS3BucketWorkflow deletes an S3 bucket via the node agent.
func DeleteS3BucketWorkflow(ctx workflow.Context, bucketID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_buckets",
		ID:     bucketID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the bucket.
	var bucket model.S3Bucket
	err = workflow.ExecuteActivity(ctx, "GetS3BucketByID", bucketID).Get(ctx, &bucket)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return err
	}

	if bucket.ShardID == nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return fmt.Errorf("s3 bucket %s has no shard assigned", bucketID)
	}

	var tenantID string
	if bucket.TenantID != nil {
		tenantID = *bucket.TenantID
	}

	var nodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *bucket.ShardID).Get(ctx, &nodes)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return err
	}

	if len(nodes) == 0 {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return fmt.Errorf("no nodes found in S3 shard %s", *bucket.ShardID)
	}

	internalName := tenantID + "--" + bucket.Name
	nodeCtx := nodeActivityCtx(ctx, nodes[0].ID)

	err = workflow.ExecuteActivity(nodeCtx, "DeleteS3Bucket", activity.DeleteS3BucketParams{
		TenantID: tenantID,
		Name:     internalName,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_buckets", bucketID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_buckets",
		ID:     bucketID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
