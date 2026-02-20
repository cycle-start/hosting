package workflow

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateZoneRecordWorkflow writes a DNS record to the PowerDNS database.
func CreateZoneRecordWorkflow(ctx workflow.Context, params model.ZoneRecordParams) error {
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
		Table:  "zone_records",
		ID:     params.RecordID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", params.ZoneName).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", params.RecordID, err)
		return err
	}
	if domainID == 0 {
		zoneErr := fmt.Errorf("zone %q not found in DNS", params.ZoneName)
		_ = setResourceFailed(ctx, "zone_records", params.RecordID, zoneErr)
		return zoneErr
	}

	// Write the record to PowerDNS DB.
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     params.Name,
		Type:     params.Type,
		Content:  params.Content,
		TTL:      params.TTL,
		Priority: params.Priority,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", params.RecordID, err)
		return err
	}

	// If this is a custom record, deactivate any matching auto records in PowerDNS.
	// Auto records stay in core DB but are removed from PowerDNS (override).
	if params.ManagedBy == model.ManagedByCustom {
		_ = workflow.ExecuteActivity(ctx, "DeactivateAutoRecords", activity.DeactivateAutoRecordsParams{
			Name: params.Name,
			Type: params.Type,
		}).Get(ctx, nil)
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     params.RecordID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateZoneRecordWorkflow updates a DNS record in the PowerDNS database.
func UpdateZoneRecordWorkflow(ctx workflow.Context, params model.ZoneRecordParams) error {
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
		Table:  "zone_records",
		ID:     params.RecordID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", params.ZoneName).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", params.RecordID, err)
		return err
	}
	if domainID == 0 {
		zoneErr := fmt.Errorf("zone %q not found in DNS", params.ZoneName)
		_ = setResourceFailed(ctx, "zone_records", params.RecordID, zoneErr)
		return zoneErr
	}

	// Update the record in PowerDNS DB.
	err = workflow.ExecuteActivity(ctx, "UpdateDNSRecord", activity.UpdateDNSRecordParams{
		DomainID: domainID,
		Name:     params.Name,
		Type:     params.Type,
		Content:  params.Content,
		TTL:      params.TTL,
		Priority: params.Priority,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", params.RecordID, err)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     params.RecordID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteZoneRecordWorkflow removes a DNS record from the PowerDNS database.
func DeleteZoneRecordWorkflow(ctx workflow.Context, params model.ZoneRecordParams) error {
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
		Table:  "zone_records",
		ID:     params.RecordID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", params.ZoneName).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", params.RecordID, err)
		return err
	}

	// Only delete from PowerDNS if the zone exists there (domainID > 0).
	// This makes delete idempotent — if the zone was already removed from
	// PowerDNS (or never created), we skip straight to marking it deleted.
	if domainID > 0 {
		err = workflow.ExecuteActivity(ctx, "DeleteDNSRecord", activity.DeleteDNSRecordParams{
			DomainID: domainID,
			Name:     params.Name,
			Type:     params.Type,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "zone_records", params.RecordID, err)
			return err
		}
	}

	// Set status to deleted.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     params.RecordID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// If we just deleted a custom record, reactivate any matching auto records
	// that were previously overridden.
	if params.ManagedBy == model.ManagedByCustom {
		_ = workflow.ExecuteActivity(ctx, "ReactivateAutoRecords", activity.DeactivateAutoRecordsParams{
			Name: params.Name,
			Type: params.Type,
		}).Get(ctx, nil)
	}

	return nil
}
