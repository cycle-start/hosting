package activity

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/temporal"

	"github.com/edvin/hosting/internal/model"
)

func TestSendCallback_Success(t *testing.T) {
	var received model.CallbackPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &received))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewCallback()
	err := a.SendCallback(context.Background(), SendCallbackParams{
		URL: srv.URL,
		Payload: model.CallbackPayload{
			ResourceType: "database",
			ResourceID:   "db-123",
			Status:       model.StatusActive,
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "database", received.ResourceType)
	assert.Equal(t, "db-123", received.ResourceID)
	assert.Equal(t, model.StatusActive, received.Status)
}

func TestSendCallback_FailedStatus(t *testing.T) {
	var received model.CallbackPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &received))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := NewCallback()
	err := a.SendCallback(context.Background(), SendCallbackParams{
		URL: srv.URL,
		Payload: model.CallbackPayload{
			ResourceType: "database",
			ResourceID:   "db-456",
			Status:       model.StatusFailed,
			StatusMessage: "workflow failed: db creation error",
		},
	})

	require.NoError(t, err)
	assert.Equal(t, model.StatusFailed, received.Status)
	assert.Equal(t, "workflow failed: db creation error", received.StatusMessage)
}

func TestSendCallback_ClientError_NonRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	a := NewCallback()
	err := a.SendCallback(context.Background(), SendCallbackParams{
		URL: srv.URL,
		Payload: model.CallbackPayload{
			ResourceType: "database",
			ResourceID:   "db-123",
			Status:       model.StatusFailed,
		},
	})

	require.Error(t, err)
	var appErr *temporal.ApplicationError
	require.ErrorAs(t, err, &appErr)
	assert.True(t, appErr.NonRetryable())
}

func TestSendCallback_ServerError_Retryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	a := NewCallback()
	err := a.SendCallback(context.Background(), SendCallbackParams{
		URL: srv.URL,
		Payload: model.CallbackPayload{
			ResourceType: "database",
			ResourceID:   "db-123",
			Status:       model.StatusFailed,
		},
	})

	require.Error(t, err)
	// Should NOT be a non-retryable ApplicationError
	var appErr *temporal.ApplicationError
	assert.False(t, errors.As(err, &appErr))
}

func TestSendCallback_Unreachable_Retryable(t *testing.T) {
	a := NewCallback()
	err := a.SendCallback(context.Background(), SendCallbackParams{
		URL: "http://127.0.0.1:1",
		Payload: model.CallbackPayload{
			ResourceType: "database",
			ResourceID:   "db-123",
			Status:       model.StatusActive,
		},
	})

	require.Error(t, err)
	// Network error — should be retryable (not a non-retryable ApplicationError)
	var appErr *temporal.ApplicationError
	assert.False(t, errors.As(err, &appErr))
}
