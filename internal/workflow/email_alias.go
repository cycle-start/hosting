package workflow

import (
	"encoding/json"
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
			MaximumAttempts: 3,
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
		_ = setResourceFailed(ctx, "email_aliases", aliasID)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", alias.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID)
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
		_ = setResourceFailed(ctx, "email_aliases", aliasID)
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
			MaximumAttempts: 3,
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
		_ = setResourceFailed(ctx, "email_aliases", aliasID)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", alias.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_aliases", aliasID)
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
		_ = setResourceFailed(ctx, "email_aliases", aliasID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_aliases",
		ID:     aliasID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}

// resolveClusterStalwart traverses FQDN → webroot → tenant → cluster to get
// Stalwart URL and token. On failure, it sets the resource to failed.
func resolveClusterStalwart(ctx workflow.Context, fqdnID, table, resourceID string) (baseURL, adminToken string, err error) {
	var fqdn model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNByID", fqdnID).Get(ctx, &fqdn)
	if err != nil {
		_ = setResourceFailed(ctx, table, resourceID)
		return "", "", err
	}

	var webroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", fqdn.WebrootID).Get(ctx, &webroot)
	if err != nil {
		_ = setResourceFailed(ctx, table, resourceID)
		return "", "", err
	}

	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", webroot.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, table, resourceID)
		return "", "", err
	}

	var cluster model.Cluster
	err = workflow.ExecuteActivity(ctx, "GetClusterByID", tenant.ClusterID).Get(ctx, &cluster)
	if err != nil {
		_ = setResourceFailed(ctx, table, resourceID)
		return "", "", err
	}

	var cfg struct {
		StalwartURL   string `json:"stalwart_url"`
		StalwartToken string `json:"stalwart_token"`
	}
	_ = json.Unmarshal(cluster.Config, &cfg)

	return cfg.StalwartURL, cfg.StalwartToken, nil
}
