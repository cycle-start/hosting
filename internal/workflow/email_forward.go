package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateEmailForwardWorkflow provisions an email forward by regenerating
// the Sieve forwarding script via JMAP.
func CreateEmailForwardWorkflow(ctx workflow.Context, forwardID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_forwards",
		ID:     forwardID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the forward.
	var fwd model.EmailForward
	err = workflow.ExecuteActivity(ctx, "GetEmailForwardByID", forwardID).Get(ctx, &fwd)
	if err != nil {
		_ = setResourceFailed(ctx, "email_forwards", forwardID, err)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", fwd.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_forwards", forwardID, err)
		return err
	}

	// Resolve Stalwart credentials.
	baseURL, adminToken, err := resolveClusterStalwart(ctx, account.FQDNID, "email_forwards", forwardID)
	if err != nil {
		return err
	}

	// Sync the forwarding Sieve script.
	err = workflow.ExecuteActivity(ctx, "StalwartSyncForwardScript", activity.StalwartSyncForwardParams{
		BaseURL:        baseURL,
		AdminToken:     adminToken,
		AccountName:    account.Address,
		EmailAccountID: account.ID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_forwards", forwardID, err)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_forwards",
		ID:     forwardID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteEmailForwardWorkflow removes an email forward and regenerates the
// Sieve forwarding script.
func DeleteEmailForwardWorkflow(ctx workflow.Context, forwardID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_forwards",
		ID:     forwardID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the forward.
	var fwd model.EmailForward
	err = workflow.ExecuteActivity(ctx, "GetEmailForwardByID", forwardID).Get(ctx, &fwd)
	if err != nil {
		_ = setResourceFailed(ctx, "email_forwards", forwardID, err)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", fwd.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_forwards", forwardID, err)
		return err
	}

	// Resolve Stalwart credentials.
	baseURL, adminToken, err := resolveClusterStalwart(ctx, account.FQDNID, "email_forwards", forwardID)
	if err != nil {
		return err
	}

	// Set status to deleted first so the sync excludes this forward.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_forwards",
		ID:     forwardID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Sync the forwarding Sieve script (without the deleted forward).
	err = workflow.ExecuteActivity(ctx, "StalwartSyncForwardScript", activity.StalwartSyncForwardParams{
		BaseURL:        baseURL,
		AdminToken:     adminToken,
		AccountName:    account.Address,
		EmailAccountID: account.ID,
	}).Get(ctx, nil)
	if err != nil {
		// Forward is already marked deleted; sync failure is best-effort.
		return err
	}

	return nil
}
