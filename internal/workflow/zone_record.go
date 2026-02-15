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
func CreateZoneRecordWorkflow(ctx workflow.Context, recordID string) error {
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
		ID:     recordID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone record and zone name.
	var zctx activity.ZoneRecordContext
	err = workflow.ExecuteActivity(ctx, "GetZoneRecordContext", recordID).Get(ctx, &zctx)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zctx.ZoneName).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
		return err
	}
	if domainID == 0 {
		zoneErr := fmt.Errorf("zone %q not found in DNS", zctx.ZoneName)
		_ = setResourceFailed(ctx, "zone_records", recordID, zoneErr)
		return zoneErr
	}

	// Write the record to PowerDNS DB.
	err = workflow.ExecuteActivity(ctx, "WriteDNSRecord", activity.WriteDNSRecordParams{
		DomainID: domainID,
		Name:     zctx.Record.Name,
		Type:     zctx.Record.Type,
		Content:  zctx.Record.Content,
		TTL:      zctx.Record.TTL,
		Priority: zctx.Record.Priority,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
		return err
	}

	// If this is a custom record, deactivate any matching auto records in PowerDNS.
	// Auto records stay in core DB but are removed from PowerDNS (override).
	if zctx.Record.ManagedBy == model.ManagedByCustom {
		_ = workflow.ExecuteActivity(ctx, "DeactivateAutoRecords", activity.DeactivateAutoRecordsParams{
			Name: zctx.Record.Name,
			Type: zctx.Record.Type,
		}).Get(ctx, nil)
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
		ID:     recordID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone record and zone name.
	var zctx activity.ZoneRecordContext
	err = workflow.ExecuteActivity(ctx, "GetZoneRecordContext", recordID).Get(ctx, &zctx)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zctx.ZoneName).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
		return err
	}
	if domainID == 0 {
		zoneErr := fmt.Errorf("zone %q not found in DNS", zctx.ZoneName)
		_ = setResourceFailed(ctx, "zone_records", recordID, zoneErr)
		return zoneErr
	}

	// Update the record in PowerDNS DB.
	err = workflow.ExecuteActivity(ctx, "UpdateDNSRecord", activity.UpdateDNSRecordParams{
		DomainID: domainID,
		Name:     zctx.Record.Name,
		Type:     zctx.Record.Type,
		Content:  zctx.Record.Content,
		TTL:      zctx.Record.TTL,
		Priority: zctx.Record.Priority,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
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
		ID:     recordID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the zone record and zone name.
	var zctx activity.ZoneRecordContext
	err = workflow.ExecuteActivity(ctx, "GetZoneRecordContext", recordID).Get(ctx, &zctx)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
		return err
	}

	// Get the PowerDNS domain ID.
	var domainID int
	err = workflow.ExecuteActivity(ctx, "GetDNSZoneIDByName", zctx.ZoneName).Get(ctx, &domainID)
	if err != nil {
		_ = setResourceFailed(ctx, "zone_records", recordID, err)
		return err
	}

	// Only delete from PowerDNS if the zone exists there (domainID > 0).
	// This makes delete idempotent — if the zone was already removed from
	// PowerDNS (or never created), we skip straight to marking it deleted.
	if domainID > 0 {
		err = workflow.ExecuteActivity(ctx, "DeleteDNSRecord", activity.DeleteDNSRecordParams{
			DomainID: domainID,
			Name:     zctx.Record.Name,
			Type:     zctx.Record.Type,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "zone_records", recordID, err)
			return err
		}
	}

	// Capture record info before deletion for reactivation check.
	recordName := zctx.Record.Name
	recordType := zctx.Record.Type
	recordManagedBy := zctx.Record.ManagedBy

	// Set status to deleted.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "zone_records",
		ID:     recordID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// If we just deleted a custom record, reactivate any matching auto records
	// that were previously overridden.
	if recordManagedBy == model.ManagedByCustom {
		_ = workflow.ExecuteActivity(ctx, "ReactivateAutoRecords", activity.DeactivateAutoRecordsParams{
			Name: recordName,
			Type: recordType,
		}).Get(ctx, nil)
	}

	return nil
}
