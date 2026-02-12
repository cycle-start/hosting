package stalwart

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJMAPClient_DeploySieveScript_Success(t *testing.T) {
	var uploadCalled, jmapCalled bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		if r.URL.Path == "/jmap/upload/user@example.com/" {
			uploadCalled = true
			assert.Equal(t, http.MethodPost, r.Method)
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), "redirect")
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"blobId": "blob-123"})
			return
		}

		if r.URL.Path == "/jmap" {
			jmapCalled = true
			assert.Equal(t, http.MethodPost, r.Method)
			var req map[string]any
			json.NewDecoder(r.Body).Decode(&req)
			calls := req["methodCalls"].([]any)
			call := calls[0].([]any)
			assert.Equal(t, "SieveScript/set", call[0])
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
			return
		}

		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeploySieveScript(context.Background(), srv.URL, "test-token", "user@example.com", "test-script", "redirect \"bob@gmail.com\";")
	require.NoError(t, err)
	assert.True(t, uploadCalled)
	assert.True(t, jmapCalled)
}

func TestJMAPClient_DeploySieveScript_UploadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("upload failed"))
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeploySieveScript(context.Background(), srv.URL, "test-token", "user@example.com", "test-script", "content")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "upload sieve blob")
}

func TestJMAPClient_DeleteSieveScript_Success(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// Query response.
			json.NewEncoder(w).Encode(map[string]any{
				"methodResponses": []any{
					[]any{"SieveScript/query", map[string]any{"ids": []any{"script-1"}}, "0"},
				},
			})
			return
		}

		// Destroy response.
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		assert.Equal(t, "SieveScript/set", call[0])
		json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeleteSieveScript(context.Background(), srv.URL, "test-token", "user@example.com", "test-script")
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func TestJMAPClient_DeleteSieveScript_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"methodResponses": []any{
				[]any{"SieveScript/query", map[string]any{"ids": []any{}}, "0"},
			},
		})
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeleteSieveScript(context.Background(), srv.URL, "test-token", "user@example.com", "test-script")
	require.NoError(t, err)
}

func TestJMAPClient_SetVacationResponse_Enable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jmap", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		assert.Equal(t, "VacationResponse/set", call[0])
		args := call[1].(map[string]any)
		update := args["update"].(map[string]any)
		singleton := update["singleton"].(map[string]any)
		assert.Equal(t, true, singleton["isEnabled"])
		assert.Equal(t, "Out of office", singleton["subject"])
		assert.Equal(t, "I'm on vacation.", singleton["textBody"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.SetVacationResponse(context.Background(), srv.URL, "test-token", "user@example.com", &VacationParams{
		Subject: "Out of office",
		Body:    "I'm on vacation.",
		Enabled: true,
	})
	require.NoError(t, err)
}

func TestJMAPClient_SetVacationResponse_Disable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		args := call[1].(map[string]any)
		update := args["update"].(map[string]any)
		singleton := update["singleton"].(map[string]any)
		assert.Equal(t, false, singleton["isEnabled"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.SetVacationResponse(context.Background(), srv.URL, "test-token", "user@example.com", nil)
	require.NoError(t, err)
}
