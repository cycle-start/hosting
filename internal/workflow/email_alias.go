package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateEmailAliasWorkflow provisions an email alias in Stalwart by adding
// the address to the principal's emails array.
func CreateEmailAliasWorkflow(ctx workflow.Context, aliasID string) error {
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
		Table:  "email_aliases",
		ID:     aliasID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the alias.
	var alias model.EmailAlias
	err = workflow.ExecuteActivity(ctx, "GetEmailAliasByID", aliasID).Get(ctx, &alias)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID, err)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", alias.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID, err)
		return err
	}

	// Traverse hierarchy: FQDN → webroot → tenant → cluster.
	baseURL, adminToken, err := resolveClusterStalwart(ctx, account.FQDNID, "email_aliases", aliasID)
	if err != nil {
		return err
	}

	// Add alias to Stalwart principal.
	err = workflow.ExecuteActivity(ctx, "StalwartAddAlias", activity.StalwartAliasParams{
		BaseURL:     baseURL,
		AdminToken:  adminToken,
		AccountName: account.Address,
		Address:     alias.Address,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID, err)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_aliases",
		ID:     aliasID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteEmailAliasWorkflow removes an email alias from Stalwart.
func DeleteEmailAliasWorkflow(ctx workflow.Context, aliasID string) error {
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
		Table:  "email_aliases",
		ID:     aliasID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the alias.
	var alias model.EmailAlias
	err = workflow.ExecuteActivity(ctx, "GetEmailAliasByID", aliasID).Get(ctx, &alias)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID, err)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", alias.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID, err)
		return err
	}

	// Traverse hierarchy for Stalwart credentials.
	baseURL, adminToken, err := resolveClusterStalwart(ctx, account.FQDNID, "email_aliases", aliasID)
	if err != nil {
		return err
	}

	// Remove alias from Stalwart principal.
	err = workflow.ExecuteActivity(ctx, "StalwartRemoveAlias", activity.StalwartAliasParams{
		BaseURL:     baseURL,
		AdminToken:  adminToken,
		AccountName: account.Address,
		Address:     alias.Address,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID, err)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_aliases",
		ID:     aliasID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}

// resolveClusterStalwart uses GetStalwartContext to resolve Stalwart URL and
// token for a given FQDN. On failure, it sets the resource to failed.
func resolveClusterStalwart(ctx workflow.Context, fqdnID, table, resourceID string) (baseURL, adminToken string, err error) {
	var sctx activity.StalwartContext
	err = workflow.ExecuteActivity(ctx, "GetStalwartContext", fqdnID).Get(ctx, &sctx)
	if err != nil {
		_ = setResourceFailed(ctx, table, resourceID, err)
		return "", "", err
	}
	return sctx.StalwartURL, sctx.StalwartToken, nil
}
