package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateZoneWorkflow creates a DNS zone in the service DB (PowerDNS)
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
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Get platform config for SOA and NS records.
	var primaryNS string
	err = workflow.ExecuteActivity(ctx, "GetPlatformConfig", "primary_ns").Get(ctx, &primaryNS)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	var hostmasterEmail string
	err = workflow.ExecuteActivity(ctx, "GetPlatformConfig", "hostmaster_email").Get(ctx, &hostmasterEmail)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	var secondaryNS string
	err = workflow.ExecuteActivity(ctx, "GetPlatformConfig", "secondary_ns").Get(ctx, &secondaryNS)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Write zone to service DB (PowerDNS domains table).
	var domainID int
	err = workflow.ExecuteActivity(ctx, "WriteDNSZone", activity.WriteDNSZoneParams{
		Name: zone.Name,
		Type: "NATIVE",
	}).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Create SOA record.
	soaContent := fmt.Sprintf("%s %s 1 10800 3600 604800 300", primaryNS, hostmasterEmail)
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     zone.Name,
		Type:     "SOA",
		Content:  soaContent,
		TTL:      86400,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Create primary NS record.
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     zone.Name,
		Type:     "NS",
		Content:  primaryNS,
		TTL:      86400,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Create secondary NS record.
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     zone.Name,
		Type:     "NS",
		Content:  secondaryNS,
		TTL:      86400,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zones",
		ID:     zoneID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteZoneWorkflow removes a DNS zone from the service DB (PowerDNS).
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
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zone.Name).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Delete all records for this domain.
	err = workflow.ExecuteActivity(ctx, "DeleteDNSRecordsByDomain", domainID).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Delete the zone from service DB.
	err = workflow.ExecuteActivity(ctx, "DeleteDNSZone", zone.Name).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zones", zoneID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zones",
		ID:     zoneID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
