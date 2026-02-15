package workflow

import (
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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
		return err
	}

	// Resolve Stalwart context (FQDN → webroot → tenant → cluster in one query).
	var sctx activity.StalwartContext
	err = workflow.ExecuteActivity(ctx, "GetStalwartContext", account.FQDNID).Get(ctx, &sctx)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
		return err
	}

	// Create domain in Stalwart (idempotent).
	err = workflow.ExecuteActivity(ctx, "StalwartCreateDomain", activity.StalwartDomainParams{
		BaseURL:    sctx.StalwartURL,
		AdminToken: sctx.StalwartToken,
		Domain:     sctx.FQDN,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
		return err
	}

	// Create account in Stalwart.
	err = workflow.ExecuteActivity(ctx, "StalwartCreateAccount", activity.StalwartCreateAccountParams{
		BaseURL:     sctx.StalwartURL,
		AdminToken:  sctx.StalwartToken,
		Address:     account.Address,
		DisplayName: account.DisplayName,
		QuotaBytes:  account.QuotaBytes,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
		return err
	}

	// Auto-create email DNS records (MX, SPF, DKIM, DMARC) if a zone exists.
	mailHostname := sctx.MailHostname
	if mailHostname == "" {
		mailHostname = "mail." + sctx.FQDN
	}
	err = workflow.ExecuteActivity(ctx, "AutoCreateEmailDNSRecords", activity.AutoCreateEmailDNSRecordsParams{
		FQDN:          sctx.FQDN,
		MailHostname:  mailHostname,
		SPFIncludes:   sctx.SPFIncludes,
		DKIMSelector:  sctx.DKIMSelector,
		DKIMPublicKey: sctx.DKIMPublicKey,
		DMARCPolicy:   sctx.DMARCPolicy,
		SourceFQDNID:  sctx.FQDNID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
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
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
		return err
	}

	// Resolve Stalwart context (FQDN → webroot → tenant → cluster in one query).
	var sctx activity.StalwartContext
	err = workflow.ExecuteActivity(ctx, "GetStalwartContext", account.FQDNID).Get(ctx, &sctx)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
		return err
	}

	// Delete account from Stalwart.
	err = workflow.ExecuteActivity(ctx, "StalwartDeleteAccount", activity.StalwartDeleteAccountParams{
		BaseURL:    sctx.StalwartURL,
		AdminToken: sctx.StalwartToken,
		Address:    account.Address,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_accounts", accountID, err)
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
			BaseURL:    sctx.StalwartURL,
			AdminToken: sctx.StalwartToken,
			Domain:     sctx.FQDN,
		}).Get(ctx, nil)

		// Delete email DNS records.
		_ = workflow.ExecuteActivity(ctx, "AutoDeleteEmailDNSRecords", sctx.FQDN).Get(ctx, nil)
	}

	return nil
}
