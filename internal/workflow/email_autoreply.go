package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/stalwart"
)

// UpdateEmailAutoReplyWorkflow deploys a vacation auto-reply via JMAP.
func UpdateEmailAutoReplyWorkflow(ctx workflow.Context, autoReplyID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_autoreplies",
		ID:     autoReplyID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the auto-reply.
	var ar model.EmailAutoReply
	err = workflow.ExecuteActivity(ctx, "GetEmailAutoReplyByID", autoReplyID).Get(ctx, &ar)
	if err != nil {
		_ = setResourceFailed(ctx, "email_autoreplies", autoReplyID)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", ar.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_autoreplies", autoReplyID)
		return err
	}

	// Resolve Stalwart credentials.
	baseURL, adminToken, err := resolveClusterStalwart(ctx, account.FQDNID, "email_autoreplies", autoReplyID)
	if err != nil {
		return err
	}

	// Build vacation params.
	vacation := &stalwart.VacationParams{
		Subject: ar.Subject,
		Body:    ar.Body,
		Enabled: ar.Enabled,
	}
	if ar.StartDate != nil {
		s := ar.StartDate.Format(time.RFC3339)
		vacation.StartDate = &s
	}
	if ar.EndDate != nil {
		s := ar.EndDate.Format(time.RFC3339)
		vacation.EndDate = &s
	}

	// Deploy vacation auto-reply via JMAP.
	err = workflow.ExecuteActivity(ctx, "StalwartSetVacation", activity.StalwartVacationParams{
		BaseURL:     baseURL,
		AdminToken:  adminToken,
		AccountName: account.Address,
		Vacation:    vacation,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_autoreplies", autoReplyID)
		return err
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_autoreplies",
		ID:     autoReplyID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteEmailAutoReplyWorkflow clears the vacation auto-reply via JMAP.
func DeleteEmailAutoReplyWorkflow(ctx workflow.Context, autoReplyID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_autoreplies",
		ID:     autoReplyID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Look up the auto-reply.
	var ar model.EmailAutoReply
	err = workflow.ExecuteActivity(ctx, "GetEmailAutoReplyByID", autoReplyID).Get(ctx, &ar)
	if err != nil {
		_ = setResourceFailed(ctx, "email_autoreplies", autoReplyID)
		return err
	}

	// Look up the email account.
	var account model.EmailAccount
	err = workflow.ExecuteActivity(ctx, "GetEmailAccountByID", ar.EmailAccountID).Get(ctx, &account)
	if err != nil {
		_ = setResourceFailed(ctx, "email_autoreplies", autoReplyID)
		return err
	}

	// Resolve Stalwart credentials.
	baseURL, adminToken, err := resolveClusterStalwart(ctx, account.FQDNID, "email_autoreplies", autoReplyID)
	if err != nil {
		return err
	}

	// Clear vacation auto-reply.
	err = workflow.ExecuteActivity(ctx, "StalwartSetVacation", activity.StalwartVacationParams{
		BaseURL:     baseURL,
		AdminToken:  adminToken,
		AccountName: account.Address,
		Vacation:    nil,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "email_autoreplies", autoReplyID)
		return err
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "email_autoreplies",
		ID:     autoReplyID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
