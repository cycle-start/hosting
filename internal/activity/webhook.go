package activity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.temporal.io/sdk/temporal"

	"github.com/edvin/hosting/internal/model"
)

// Webhook contains activities for sending incident webhook notifications.
type Webhook struct {
	client *http.Client
}

// NewWebhook creates a new Webhook activity struct.
func NewWebhook() *Webhook {
	return &Webhook{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// SendWebhookParams holds parameters for the SendWebhook activity.
type SendWebhookParams struct {
	URL      string         `json:"url"`
	Template string         `json:"template"` // "generic" or "slack"
	Incident model.Incident `json:"incident"`
	Trigger  string         `json:"trigger"` // "critical" or "escalated"
}

// SendWebhook POSTs a webhook notification for an incident.
func (a *Webhook) SendWebhook(ctx context.Context, params SendWebhookParams) error {
	var body []byte
	var err error

	switch params.Template {
	case "slack":
		body, err = buildSlackPayload(params.Incident, params.Trigger)
	default:
		body, err = buildGenericPayload(params.Incident, params.Trigger)
	}
	if err != nil {
		return temporal.NewNonRetryableApplicationError("build webhook payload", "MARSHAL_ERROR", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, params.URL, bytes.NewReader(body))
	if err != nil {
		return temporal.NewNonRetryableApplicationError("create webhook request", "REQUEST_ERROR", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook POST to %s: %w", params.URL, err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("webhook returned %d", resp.StatusCode),
			"CLIENT_ERROR", nil)
	}
	return fmt.Errorf("webhook returned %d", resp.StatusCode)
}

// GenericWebhookPayload is the default JSON payload for webhooks.
type GenericWebhookPayload struct {
	Event    string         `json:"event"`
	Incident model.Incident `json:"incident"`
}

func buildGenericPayload(inc model.Incident, trigger string) ([]byte, error) {
	return json.Marshal(GenericWebhookPayload{
		Event:    "incident." + trigger,
		Incident: inc,
	})
}

// buildSlackPayload creates a Slack Block Kit message.
func buildSlackPayload(inc model.Incident, trigger string) ([]byte, error) {
	emoji := ":warning:"
	if inc.Severity == "critical" {
		emoji = ":rotating_light:"
	}

	title := fmt.Sprintf("%s *%s*", emoji, inc.Title)

	fields := []map[string]interface{}{
		{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Status:* %s", inc.Status),
		},
		{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Severity:* %s", inc.Severity),
		},
		{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Type:* %s", inc.Type),
		},
		{
			"type": "mrkdwn",
			"text": fmt.Sprintf("*Source:* %s", inc.Source),
		},
	}

	blocks := []map[string]interface{}{
		{
			"type": "header",
			"text": map[string]string{
				"type": "plain_text",
				"text": fmt.Sprintf("Incident %s", trigger),
			},
		},
		{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": title,
			},
		},
		{
			"type":   "section",
			"fields": fields,
		},
	}

	if inc.Detail != "" {
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": fmt.Sprintf("```%s```", inc.Detail),
			},
		})
	}

	return json.Marshal(map[string]interface{}{
		"blocks": blocks,
	})
}
