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

// Callback contains activities for sending provisioning callback notifications.
type Callback struct {
	client *http.Client
}

// NewCallback creates a new Callback activity struct.
func NewCallback() *Callback {
	return &Callback{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// SendCallbackParams holds parameters for the SendCallback activity.
type SendCallbackParams struct {
	URL     string                `json:"url"`
	Payload model.CallbackPayload `json:"payload"`
}

// SendCallback POSTs a JSON payload to the given callback URL.
//   - 2xx → success (return nil)
//   - 4xx → non-retryable error (client error, don't retry)
//   - 5xx / network error → retryable error (Temporal retries)
func (a *Callback) SendCallback(ctx context.Context, params SendCallbackParams) error {
	body, err := json.Marshal(params.Payload)
	if err != nil {
		return temporal.NewNonRetryableApplicationError("marshal callback payload", "MARSHAL_ERROR", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, params.URL, bytes.NewReader(body))
	if err != nil {
		return temporal.NewNonRetryableApplicationError("create callback request", "REQUEST_ERROR", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("callback POST to %s: %w", params.URL, err)
	}
	defer func() { io.Copy(io.Discard, resp.Body); resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("callback returned %d", resp.StatusCode),
			"CLIENT_ERROR", nil)
	}
	return fmt.Errorf("callback returned %d", resp.StatusCode)
}
