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

// ProvisionLECertWorkflow provisions a Let's Encrypt certificate for an FQDN.
// The ACME challenge flow is stubbed; the certificate is created with placeholder data.
func ProvisionLECertWorkflow(ctx workflow.Context, fqdnID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Look up the FQDN.
	var fqdn model.FQDN
	err := workflow.ExecuteActivity(ctx, "GetFQDNByID", fqdnID).Get(ctx, &fqdn)
	if err != nil {
		return err
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

	// ACME challenge stub: In production, this would perform the HTTP-01 or DNS-01 challenge
	// and obtain the real certificate from Let's Encrypt. For now, we store placeholder data.
	now := workflow.Now(ctx)
	expiresAt := now.Add(90 * 24 * time.Hour) // LE certs are valid for 90 days.

	err = workflow.ExecuteActivity(ctx, "StoreCertificate", activity.StoreCertParams{
		ID:        certID,
		CertPEM:   "PLACEHOLDER_CERT_PEM",
		KeyPEM:    "PLACEHOLDER_KEY_PEM",
		ChainPEM:  "PLACEHOLDER_CHAIN_PEM",
		IssuedAt:  now,
		ExpiresAt: expiresAt,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Look up the webroot and tenant to find the shard nodes.
	var certWebroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", fqdn.WebrootID).Get(ctx, &certWebroot)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	var certTenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", certWebroot.TenantID).Get(ctx, &certTenant)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	if certTenant.ShardID == nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return fmt.Errorf("tenant %s has no shard assigned", certWebroot.TenantID)
	}

	var certNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *certTenant.ShardID).Get(ctx, &certNodes)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Install the certificate on each node in the shard.
	for _, node := range certNodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "InstallCertificate", activity.InstallCertificateParams{
			FQDN:     fqdn.FQDN,
			CertPEM:  "PLACEHOLDER_CERT_PEM",
			KeyPEM:   "PLACEHOLDER_KEY_PEM",
			ChainPEM: "PLACEHOLDER_CHAIN_PEM",
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "certificates", certID)
			return err
		}
	}

	// Deactivate other certificates for this FQDN.
	err = workflow.ExecuteActivity(ctx, "DeactivateOtherCerts", fqdnID, certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Activate this certificate.
	err = workflow.ExecuteActivity(ctx, "ActivateCertificate", certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	return nil
}

// UploadCustomCertWorkflow validates, stores, and installs a custom certificate.
func UploadCustomCertWorkflow(ctx workflow.Context, certID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
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
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Validate the certificate and key.
	err = workflow.ExecuteActivity(ctx, "ValidateCustomCert", cert.CertPEM, cert.KeyPEM).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Look up the FQDN for the certificate.
	var fqdn model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNByID", cert.FQDNID).Get(ctx, &fqdn)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Look up the webroot and tenant to find the shard nodes.
	var ucWebroot model.Webroot
	err = workflow.ExecuteActivity(ctx, "GetWebrootByID", fqdn.WebrootID).Get(ctx, &ucWebroot)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	var ucTenant model.Tenant
	err = workflow.ExecuteActivity(ctx, "GetTenantByID", ucWebroot.TenantID).Get(ctx, &ucTenant)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	if ucTenant.ShardID == nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return fmt.Errorf("tenant %s has no shard assigned", ucWebroot.TenantID)
	}

	var ucNodes []model.Node
	err = workflow.ExecuteActivity(ctx, "ListNodesByShard", *ucTenant.ShardID).Get(ctx, &ucNodes)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Install the certificate on each node in the shard.
	for _, node := range ucNodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "InstallCertificate", activity.InstallCertificateParams{
			FQDN:     fqdn.FQDN,
			CertPEM:  cert.CertPEM,
			KeyPEM:   cert.KeyPEM,
			ChainPEM: cert.ChainPEM,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "certificates", certID)
			return err
		}
	}

	// Deactivate other certificates for this FQDN.
	err = workflow.ExecuteActivity(ctx, "DeactivateOtherCerts", cert.FQDNID, certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	// Activate this certificate.
	err = workflow.ExecuteActivity(ctx, "ActivateCertificate", certID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "certificates", certID)
		return err
	}

	return nil
}

// RenewLECertWorkflow is a cron workflow that renews Let's Encrypt certificates
// expiring within 30 days.
func RenewLECertWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Stub: In production, this would query for certificates of type "lets_encrypt"
	// with expires_at within 30 days, and for each one, start a child workflow
	// to re-provision via ACME.
	//
	// var certsToRenew []model.Certificate
	// err := workflow.ExecuteActivity(ctx, "GetExpiringLECerts", 30).Get(ctx, &certsToRenew)
	// if err != nil { return err }
	// for _, cert := range certsToRenew {
	//     childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
	//         WorkflowID: "renew-le-cert-" + cert.ID.String(),
	//     })
	//     _ = workflow.ExecuteChildWorkflow(childCtx, ProvisionLECertWorkflow, cert.FQDNID).Get(ctx, nil)
	// }

	workflow.GetLogger(ctx).Info("RenewLECertWorkflow completed (stub)")
	return nil
}

// CleanupExpiredCertsWorkflow is a cron workflow that removes expired custom
// certificates that have been expired for more than 30 days.
func CleanupExpiredCertsWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Stub: In production, this would query for expired custom certificates
	// with expires_at more than 30 days ago, deactivate and delete them.
	//
	// var expiredCerts []model.Certificate
	// err := workflow.ExecuteActivity(ctx, "GetExpiredCustomCerts", 30).Get(ctx, &expiredCerts)
	// if err != nil { return err }
	// for _, cert := range expiredCerts {
	//     _ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
	//         Table:  "certificates",
	//         ID:     cert.ID,
	//         Status: model.StatusDeleted,
	//     }).Get(ctx, nil)
	// }

	workflow.GetLogger(ctx).Info("CleanupExpiredCertsWorkflow completed (stub)")
	return nil
}
