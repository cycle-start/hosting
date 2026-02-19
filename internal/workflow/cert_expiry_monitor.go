package workflow

import (
	"fmt"
	"math"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
)

// CheckCertExpiryWorkflow runs daily and creates incidents for certificates
// expiring within 14 days. Severity: warning for 7-14 days, critical for <7 days.
func CheckCertExpiryWorkflow(ctx workflow.Context) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 2,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Get all certs expiring within 14 days.
	var expiring []activity.ExpiringCert
	err := workflow.ExecuteActivity(ctx, "GetExpiringCerts", 14).Get(ctx, &expiring)
	if err != nil {
		return fmt.Errorf("get expiring certs: %w", err)
	}

	now := workflow.Now(ctx)

	for _, cert := range expiring {
		daysLeft := int(math.Ceil(cert.ExpiresAt.Sub(now).Hours() / 24))

		severity := "warning"
		if daysLeft <= 7 {
			severity = "critical"
		}

		createIncident(ctx, activity.CreateIncidentParams{
			DedupeKey:    fmt.Sprintf("cert_expiring:%s", cert.ID),
			Type:         "cert_expiring",
			Severity:     severity,
			Title:        fmt.Sprintf("Certificate %s expires in %d days", cert.ID, daysLeft),
			Detail:       fmt.Sprintf("Certificate %s (type: %s, FQDN: %s) expires at %s (%d days remaining)", cert.ID, cert.Type, cert.FQDNID, cert.ExpiresAt.Format(time.RFC3339), daysLeft),
			ResourceType: strPtr("certificate"),
			ResourceID:   &cert.ID,
			Source:       "cert-expiry-monitor-cron",
		})
	}

	// Auto-resolve cert_expiring incidents for certs that were renewed
	// (they won't appear in the expiring list anymore).
	// We need to check all active certs — but we can't list all certs easily.
	// Instead, rely on the renewal cron to renew them, and when they get a new cert,
	// the old cert becomes inactive and will be auto-resolved.
	// For simplicity, we don't auto-resolve here — the cert renewal cron handles
	// the fix, and the cert_expiring incident will stop being deduplicated once the
	// old cert is deactivated.

	return nil
}
