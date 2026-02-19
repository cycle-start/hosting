package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// sendIncidentWebhook checks platform config for a webhook URL matching the trigger
// and sends a webhook notification if configured. Errors are logged but not propagated.
func sendIncidentWebhook(ctx workflow.Context, incident model.Incident, trigger string) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Read webhook config from platform_config.
	urlKey := "webhook." + trigger + ".url"
	templateKey := "webhook." + trigger + ".template"

	var webhookURL string
	err := workflow.ExecuteActivity(ctx, "GetPlatformConfig", urlKey).Get(ctx, &webhookURL)
	if err != nil {
		// No webhook configured for this trigger -- this is normal.
		return
	}
	if webhookURL == "" {
		return
	}

	template := "generic"
	var templateValue string
	err = workflow.ExecuteActivity(ctx, "GetPlatformConfig", templateKey).Get(ctx, &templateValue)
	if err == nil && templateValue != "" {
		template = templateValue
	}

	err = workflow.ExecuteActivity(ctx, "SendWebhook", activity.SendWebhookParams{
		URL:      webhookURL,
		Template: template,
		Incident: incident,
		Trigger:  trigger,
	}).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Warn("failed to send incident webhook",
			"trigger", trigger, "incident", incident.ID, "error", err)
	}
}
