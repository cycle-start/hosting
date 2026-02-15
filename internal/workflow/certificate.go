package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

// ProvisionLECertWorkflow provisions a Let's Encrypt certificate for an FQDN
// using the ACME HTTP-01 challenge flow.
func ProvisionLECertWorkflow(ctx workflow.Context, fqdnID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Fetch FQDN context (FQDN, webroot, tenant, nodes).
	var fctx activity.FQDNContext
	err := workflow.ExecuteActivity(ctx, "GetFQDNContext", fqdnID).Get(ctx, &fctx)
	if err != nil {
		return err
	}

	if fctx.Tenant.ShardID == nil {
		return fmt.Errorf("tenant %s has no shard assigned", fctx.Tenant.ID)
	}

	// Create a certificate record in the core DB.
	// Use SideEffect to generate a UUID deterministically for replay safety.
	var certID string
	encodedID := workflow.SideEffect(ctx, func(ctx workflow.Context) interface{} {
		return platform.NewID()
	})
	if err := encodedID.Get(&certID); err != nil {
		return err
	}
	err = workflow.ExecuteActivity(ctx, "CreateCertificate", activity.CreateCertificateParams{
		ID:     certID,
		FQDNID: fqdnID,
		Type:   model.CertTypeLetsEncrypt,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Set cert status to provisioning.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "certificates",
		ID:     certID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Step 1: Create ACME order.
	var orderResult activity.ACMEOrderResult
	err = workflow.ExecuteActivity(ctx, "CreateOrder", activity.ACMEOrderParams{
		FQDN: fctx.FQDN.FQDN,
	}).Get(ctx, &orderResult)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	// Step 2: For each authorization, get the HTTP-01 challenge.
	// Typically there is one authz per domain; we handle them all.
	webrootPath := fmt.Sprintf("/var/www/storage/%s/%s/%s", fctx.Tenant.ID, fctx.Webroot.Name, fctx.Webroot.PublicFolder)

	for _, authzURL := range orderResult.AuthzURLs {
		var challengeResult activity.ACMEChallengeResult
		err = workflow.ExecuteActivity(ctx, "GetHTTP01Challenge", activity.ACMEChallengeParams{
			AuthzURL:   authzURL,
			AccountKey: orderResult.AccountKey,
		}).Get(ctx, &challengeResult)
		if err != nil {
			_ = setResourceFailed(ctx, "certificates", certID, err)
			return err
		}

		// Step 3: Place the challenge file on all shard nodes.
		for _, node := range fctx.Nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			err = workflow.ExecuteActivity(nodeCtx, "PlaceHTTP01Challenge", activity.PlaceHTTP01ChallengeParams{
				WebrootPath: webrootPath,
				Token:       challengeResult.Token,
				KeyAuth:     challengeResult.KeyAuth,
			}).Get(ctx, nil)
			if err != nil {
				_ = setResourceFailed(ctx, "certificates", certID, err)
				return err
			}
		}

		// Step 4: Tell the ACME server we're ready.
		err = workflow.ExecuteActivity(ctx, "AcceptChallenge", activity.ACMEAcceptParams{
			ChallengeURL: challengeResult.ChallengeURL,
			AccountKey:   orderResult.AccountKey,
		}).Get(ctx, nil)
		if err != nil {
			// Best-effort cleanup of challenge files.
			for _, node := range fctx.Nodes {
				nodeCtx := nodeActivityCtx(ctx, node.ID)
				_ = workflow.ExecuteActivity(nodeCtx, "CleanupHTTP01Challenge", activity.CleanupHTTP01ChallengeParams{
					WebrootPath: webrootPath,
					Token:       challengeResult.Token,
				}).Get(ctx, nil)
			}
			_ = setResourceFailed(ctx, "certificates", certID, err)
			return err
		}
	}

	// Step 5: Finalize the order and get the certificate.
	var finalizeResult activity.ACMEFinalizeResult
	err = workflow.ExecuteActivity(ctx, "FinalizeOrder", activity.ACMEFinalizeParams{
		OrderURL:   orderResult.OrderURL,
		FQDN:       fctx.FQDN.FQDN,
		AccountKey: orderResult.AccountKey,
	}).Get(ctx, &finalizeResult)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	// Step 6: Cleanup challenge files on all nodes (best effort).
	for _, authzURL := range orderResult.AuthzURLs {
		// Re-derive the token from the challenge. Since we only need the token
		// for cleanup and the authzURL loop is identical, we re-fetch.
		var cleanupChallenge activity.ACMEChallengeResult
		_ = workflow.ExecuteActivity(ctx, "GetHTTP01Challenge", activity.ACMEChallengeParams{
			AuthzURL:   authzURL,
			AccountKey: orderResult.AccountKey,
		}).Get(ctx, &cleanupChallenge)

		for _, node := range fctx.Nodes {
			nodeCtx := nodeActivityCtx(ctx, node.ID)
			_ = workflow.ExecuteActivity(nodeCtx, "CleanupHTTP01Challenge", activity.CleanupHTTP01ChallengeParams{
				WebrootPath: webrootPath,
				Token:       cleanupChallenge.Token,
			}).Get(ctx, nil)
		}
	}

	// Step 7: Store the real certificate data.
	err = workflow.ExecuteActivity(ctx, "StoreCertificate", activity.StoreCertParams{
		ID:        certID,
		CertPEM:   finalizeResult.CertPEM,
		KeyPEM:    finalizeResult.KeyPEM,
		ChainPEM:  finalizeResult.ChainPEM,
		IssuedAt:  finalizeResult.IssuedAt,
		ExpiresAt: finalizeResult.ExpiresAt,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	// Step 8: Install the certificate on each node in the shard.
	for _, node := range fctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "InstallCertificate", activity.InstallCertificateParams{
			FQDN:     fctx.FQDN.FQDN,
			CertPEM:  finalizeResult.CertPEM,
			KeyPEM:   finalizeResult.KeyPEM,
			ChainPEM: finalizeResult.ChainPEM,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "certificates", certID, err)
			return err
		}
	}

	// Step 9: Deactivate other certificates for this FQDN.
	err = workflow.ExecuteActivity(ctx, "DeactivateOtherCerts", fqdnID, certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	// Step 10: Activate this certificate.
	err = workflow.ExecuteActivity(ctx, "ActivateCertificate", certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	return nil
}

// UploadCustomCertWorkflow validates, stores, and installs a custom certificate.
func UploadCustomCertWorkflow(ctx workflow.Context, certID string) error {
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

	// Set cert status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "certificates",
		ID:     certID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the certificate.
	var cert model.Certificate
	err = workflow.ExecuteActivity(ctx, "GetCertificateByID", certID).Get(ctx, &cert)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	// Validate the certificate and key.
	err = workflow.ExecuteActivity(ctx, "ValidateCustomCert", cert.CertPEM, cert.KeyPEM).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	// Fetch FQDN context (FQDN, webroot, tenant, nodes).
	var fctx activity.FQDNContext
	err = workflow.ExecuteActivity(ctx, "GetFQDNContext", cert.FQDNID).Get(ctx, &fctx)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	if fctx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", fctx.Tenant.ID)
		_ = setResourceFailed(ctx, "certificates", certID, noShardErr)
		return noShardErr
	}

	// Install the certificate on each node in the shard.
	for _, node := range fctx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "InstallCertificate", activity.InstallCertificateParams{
			FQDN:     fctx.FQDN.FQDN,
			CertPEM:  cert.CertPEM,
			KeyPEM:   cert.KeyPEM,
			ChainPEM: cert.ChainPEM,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "certificates", certID, err)
			return err
		}
	}

	// Deactivate other certificates for this FQDN.
	err = workflow.ExecuteActivity(ctx, "DeactivateOtherCerts", cert.FQDNID, certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	// Activate this certificate.
	err = workflow.ExecuteActivity(ctx, "ActivateCertificate", certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID, err)
		return err
	}

	return nil
}

// RenewLECertWorkflow is a cron workflow that renews Let's Encrypt certificates
// expiring within 30 days. For each expiring cert it starts a child
// ProvisionLECertWorkflow to issue a fresh certificate via ACME.
func RenewLECertWorkflow(ctx workflow.Context) error {
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

	var certsToRenew []model.Certificate
	err := workflow.ExecuteActivity(ctx, "GetExpiringLECerts", 30).Get(ctx, &certsToRenew)
	if err != nil {
		return err
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("found expiring LE certificates", "count", len(certsToRenew))

	for _, cert := range certsToRenew {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "renew-le-cert-" + cert.ID,
		})
		err := workflow.ExecuteChildWorkflow(childCtx, ProvisionLECertWorkflow, cert.FQDNID).Get(ctx, nil)
		if err != nil {
			logger.Error("failed to renew certificate", "certID", cert.ID, "fqdnID", cert.FQDNID, "error", err)
			// Continue renewing other certs even if one fails.
		}
	}

	return nil
}

// CleanupExpiredCertsWorkflow is a cron workflow that removes certificates
// that have been expired for more than 30 days. It deactivates them and
// clears their PEM data.
func CleanupExpiredCertsWorkflow(ctx workflow.Context) error {
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

	var expiredCerts []model.Certificate
	err := workflow.ExecuteActivity(ctx, "GetExpiredCerts", 30).Get(ctx, &expiredCerts)
	if err != nil {
		return err
	}

	logger := workflow.GetLogger(ctx)
	logger.Info("found expired certificates to clean up", "count", len(expiredCerts))

	for _, cert := range expiredCerts {
		err := workflow.ExecuteActivity(ctx, "DeleteCertificate", cert.ID).Get(ctx, nil)
		if err != nil {
			logger.Error("failed to delete expired certificate", "certID", cert.ID, "error", err)
			// Continue cleaning up other certs even if one fails.
		}
	}

	return nil
}
