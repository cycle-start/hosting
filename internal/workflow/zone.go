package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateZoneWorkflow creates a DNS zone in the PowerDNS database
// along with default SOA and NS records.
func CreateZoneWorkflow(ctx workflow.Context, zoneID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zones",
		ID:     zoneID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone.
	var zone model.Zone
	err = workflow.ExecuteActivity(ctx, "GetZoneByID", zoneID).Get(ctx, &zone)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Get brand for DNS settings.
	var brand model.Brand
	err = workflow.ExecuteActivity(ctx, "GetBrandByID", zone.BrandID).Get(ctx, &brand)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Write zone to PowerDNS DB (PowerDNS domains table).
	var domainID int
	err = workflow.ExecuteActivity(ctx, "WriteDNSZone", activity.WriteDNSZoneParams{
		Name: zone.Name,
		Type: "NATIVE",
	}).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Create SOA record.
	soaContent := fmt.Sprintf("%s %s 1 10800 3600 604800 300", brand.PrimaryNS, brand.HostmasterEmail)
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     zone.Name,
		Type:     "SOA",
		Content:  soaContent,
		TTL:      86400,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Create primary NS record.
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     zone.Name,
		Type:     "NS",
		Content:  brand.PrimaryNS,
		TTL:      86400,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Create secondary NS record.
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     zone.Name,
		Type:     "NS",
		Content:  brand.SecondaryNS,
		TTL:      86400,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zones",
		ID:     zoneID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteZoneWorkflow removes a DNS zone from the PowerDNS database.
func DeleteZoneWorkflow(ctx workflow.Context, zoneID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zones",
		ID:     zoneID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone.
	var zone model.Zone
	err = workflow.ExecuteActivity(ctx, "GetZoneByID", zoneID).Get(ctx, &zone)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Get the PowerDNS domain ID. Returns 0 if the zone doesn't exist in PowerDNS.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zone.Name).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID, err)
		return err
	}

	// Only delete from PowerDNS if the zone exists there (domainID > 0).
	// This makes delete idempotent — if the zone was already removed from
	// PowerDNS (or never created), we skip straight to marking it deleted.
	if domainID > 0 {
		// Delete all records for this domain.
		err = workflow.ExecuteActivity(ctx, "DeleteDNSRecordsByDomain", domainID).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "zones", zoneID, err)
			return err
		}

		// Delete the zone from PowerDNS DB.
		err = workflow.ExecuteActivity(ctx, "DeleteDNSZone", zone.Name).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "zones", zoneID, err)
			return err
		}
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zones",
		ID:     zoneID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
