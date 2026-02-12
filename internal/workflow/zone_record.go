package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateZoneRecordWorkflow writes a DNS record to the PowerDNS database.
func CreateZoneRecordWorkflow(ctx workflow.Context, recordID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     recordID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone record.
	var record model.ZoneRecord
	err = workflow.ExecuteActivity(ctx, "GetZoneRecordByID", recordID).Get(ctx, &record)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Look up the zone to get the zone name.
	var zone model.Zone
	err = workflow.ExecuteActivity(ctx, "GetZoneByID", record.ZoneID).Get(ctx, &zone)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zone.Name).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Write the record to PowerDNS DB.
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     record.Name,
		Type:     record.Type,
		Content:  record.Content,
		TTL:      record.TTL,
		Priority: record.Priority,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     recordID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateZoneRecordWorkflow updates a DNS record in the PowerDNS database.
func UpdateZoneRecordWorkflow(ctx workflow.Context, recordID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     recordID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone record.
	var record model.ZoneRecord
	err = workflow.ExecuteActivity(ctx, "GetZoneRecordByID", recordID).Get(ctx, &record)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Look up the zone to get the zone name.
	var zone model.Zone
	err = workflow.ExecuteActivity(ctx, "GetZoneByID", record.ZoneID).Get(ctx, &zone)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zone.Name).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Update the record in PowerDNS DB.
	err = workflow.ExecuteActivity(ctx, "UpdateDNSRecord", activity.UpdateDNSRecordParams{
		DomainID: domainID,
		Name:     record.Name,
		Type:     record.Type,
		Content:  record.Content,
		TTL:      record.TTL,
		Priority: record.Priority,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     recordID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteZoneRecordWorkflow removes a DNS record from the PowerDNS database.
func DeleteZoneRecordWorkflow(ctx workflow.Context, recordID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     recordID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone record.
	var record model.ZoneRecord
	err = workflow.ExecuteActivity(ctx, "GetZoneRecordByID", recordID).Get(ctx, &record)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Look up the zone to get the zone name.
	var zone model.Zone
	err = workflow.ExecuteActivity(ctx, "GetZoneByID", record.ZoneID).Get(ctx, &zone)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zone.Name).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Delete the record from PowerDNS DB.
	err = workflow.ExecuteActivity(ctx, "DeleteDNSRecord", activity.DeleteDNSRecordParams{
		DomainID: domainID,
		Name:     record.Name,
		Type:     record.Type,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     recordID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
