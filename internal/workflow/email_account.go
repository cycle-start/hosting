package workflow

import (
	"encoding/json"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateEmailAccountWorkflow provisions an email account in Stalwart.
// It discovers the cluster via: FQDN → webroot → tenant → cluster.
func CreateEmailAccountWorkflow(ctx workflow.Context, accountID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_accounts",
		ID:     accountID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", accountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Look up the FQDN.
	var fqdn model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNByID", account.FQDNID).Get(ctx, &fqdn)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Look up the webroot.
	var webroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", fqdn.WebrootID).Get(ctx, &webroot)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", webroot.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Get the cluster config.
	var cluster model.Cluster
	err = workflow.ExecuteActivity(ctx, "GetClusterByID", tenant.ClusterID).Get(ctx, &cluster)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	var clusterCfg struct {
		StalwartURL   string `json:"stalwart_url"`
		StalwartToken string `json:"stalwart_token"`
		MailHostname  string `json:"mail_hostname"`
	}
	_ = json.Unmarshal(cluster.Config, &clusterCfg)

	// Extract domain from email address.
	domain := fqdn.FQDN

	// Create domain in Stalwart (idempotent).
	err = workflow.ExecuteActivity(ctx, "StalwartCreateDomain", activity.StalwartDomainParams{
		BaseURL:    clusterCfg.StalwartURL,
		AdminToken: clusterCfg.StalwartToken,
		Domain:     domain,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Create account in Stalwart.
	err = workflow.ExecuteActivity(ctx, "StalwartCreateAccount", activity.StalwartCreateAccountParams{
		BaseURL:     clusterCfg.StalwartURL,
		AdminToken:  clusterCfg.StalwartToken,
		Address:     account.Address,
		DisplayName: account.DisplayName,
		QuotaBytes:  account.QuotaBytes,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Auto-create email DNS records (MX, SPF) if a zone exists.
	mailHostname := clusterCfg.MailHostname
	if mailHostname == "" {
		mailHostname = "mail." + domain
	}
	err = workflow.ExecuteActivity(ctx, "AutoCreateEmailDNSRecords", activity.AutoCreateEmailDNSRecordsParams{
		FQDN:         domain,
		MailHostname: mailHostname,
		SourceFQDNID: fqdn.ID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_accounts",
		ID:     accountID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteEmailAccountWorkflow removes an email account from Stalwart.
// If this is the last account for the FQDN, also removes the Stalwart domain
// and email DNS records.
func DeleteEmailAccountWorkflow(ctx workflow.Context, accountID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_accounts",
		ID:     accountID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", accountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Look up the FQDN.
	var fqdn model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNByID", account.FQDNID).Get(ctx, &fqdn)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Look up the webroot.
	var webroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", fqdn.WebrootID).Get(ctx, &webroot)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Look up the tenant.
	var tenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", webroot.TenantID).Get(ctx, &tenant)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Get the cluster config.
	var cluster model.Cluster
	err = workflow.ExecuteActivity(ctx, "GetClusterByID", tenant.ClusterID).Get(ctx, &cluster)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	var clusterCfg struct {
		StalwartURL   string `json:"stalwart_url"`
		StalwartToken string `json:"stalwart_token"`
	}
	_ = json.Unmarshal(cluster.Config, &clusterCfg)

	// Delete account from Stalwart.
	err = workflow.ExecuteActivity(ctx, "StalwartDeleteAccount", activity.StalwartDeleteAccountParams{
		BaseURL:    clusterCfg.StalwartURL,
		AdminToken: clusterCfg.StalwartToken,
		Address:    account.Address,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID)
		return err
	}

	// Set status to deleted (before counting, so this account is excluded).
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_accounts",
		ID:     accountID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Check if any other active email accounts exist for this FQDN.
	var remaining int
	err = workflow.ExecuteActivity(ctx, "CountActiveEmailAccountsByFQDN", account.FQDNID).Get(ctx, &remaining)
	if err != nil {
		return err
	}

	if remaining == 0 {
		// Delete domain from Stalwart.
		_ = workflow.ExecuteActivity(ctx, "StalwartDeleteDomain", activity.StalwartDomainParams{
			BaseURL:    clusterCfg.StalwartURL,
			AdminToken: clusterCfg.StalwartToken,
			Domain:     fqdn.FQDN,
		}).Get(ctx, nil)

		// Delete email DNS records.
		_ = workflow.ExecuteActivity(ctx, "AutoDeleteEmailDNSRecords", fqdn.FQDN).Get(ctx, nil)
	}

	return nil
}
