package agent

import (
	"context"

	"github.com/edvin/hosting/internal/model"
)

func (r *Reconciler) reconcileStorage(ctx context.Context, ds *model.DesiredState) ([]DriftEvent, error) {
	var events []DriftEvent

	// S3 bucket reconciliation is mostly report-only because:
	// 1. Creating buckets requires RGW admin access which the S3Manager handles
	//    but S3Manager is not wired into Server (it's created separately in main.go).
	// 2. Deleting orphaned buckets is unsafe (data loss).
	// The manager's CreateBucket is idempotent so we could safely call it, but
	// we would need the S3Manager reference. For now, just log the desired state.

	r.logger.Info().
		Int("desired_buckets", len(ds.S3Buckets)).
		Msg("storage reconciliation (report-only)")

	return events, nil
}
