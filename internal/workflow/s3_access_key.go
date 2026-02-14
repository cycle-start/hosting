package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateS3AccessKeyWorkflow provisions a new S3 access key via the node agent.
func CreateS3AccessKeyWorkflow(ctx workflow.Context, keyID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_access_keys",
		ID:     keyID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the key, bucket, and nodes.
	var sctx activity.S3AccessKeyContext
	err = workflow.ExecuteActivity(ctx, "GetS3AccessKeyContext", keyID).Get(ctx, &sctx)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, err)
		return err
	}

	if sctx.Bucket.ShardID == nil {
		noShardErr := fmt.Errorf("s3 bucket %s has no shard assigned", sctx.Key.S3BucketID)
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, noShardErr)
		return noShardErr
	}

	var tenantID string
	if sctx.Bucket.TenantID != nil {
		tenantID = *sctx.Bucket.TenantID
	}

	if len(sctx.Nodes) == 0 {
		noNodesErr := fmt.Errorf("no nodes found in S3 shard %s", *sctx.Bucket.ShardID)
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, noNodesErr)
		return noNodesErr
	}

	nodeCtx := nodeActivityCtx(ctx, sctx.Nodes[0].ID)

	err = workflow.ExecuteActivity(nodeCtx, "CreateS3AccessKey", activity.CreateS3AccessKeyParams{
		TenantID:        tenantID,
		AccessKeyID:     sctx.Key.AccessKeyID,
		SecretAccessKey: sctx.Key.SecretAccessKey,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, err)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_access_keys",
		ID:     keyID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteS3AccessKeyWorkflow deletes an S3 access key via the node agent.
func DeleteS3AccessKeyWorkflow(ctx workflow.Context, keyID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_access_keys",
		ID:     keyID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the key, bucket, and nodes.
	var sctx activity.S3AccessKeyContext
	err = workflow.ExecuteActivity(ctx, "GetS3AccessKeyContext", keyID).Get(ctx, &sctx)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, err)
		return err
	}

	if sctx.Bucket.ShardID == nil {
		noShardErr := fmt.Errorf("s3 bucket %s has no shard assigned", sctx.Key.S3BucketID)
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, noShardErr)
		return noShardErr
	}

	var tenantID string
	if sctx.Bucket.TenantID != nil {
		tenantID = *sctx.Bucket.TenantID
	}

	if len(sctx.Nodes) == 0 {
		noNodesErr := fmt.Errorf("no nodes found in S3 shard %s", *sctx.Bucket.ShardID)
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, noNodesErr)
		return noNodesErr
	}

	nodeCtx := nodeActivityCtx(ctx, sctx.Nodes[0].ID)

	err = workflow.ExecuteActivity(nodeCtx, "DeleteS3AccessKey", activity.DeleteS3AccessKeyParams{
		TenantID:    tenantID,
		AccessKeyID: sctx.Key.AccessKeyID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "s3_access_keys", keyID, err)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "s3_access_keys",
		ID:     keyID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
