package stalwart

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJMAPClient_DeploySieveScript_Create(t *testing.T) {
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "expected basic auth")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "test-token", pass)
		assert.Equal(t, "/jmap", r.URL.Path)

		callCount++
		w.Header().Set("Content-Type", "application/json")

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		method := call[0].(string)

		switch callCount {
		case 1:
			// Blob/upload
			assert.Equal(t, "Blob/upload", method)
			json.NewEncoder(w).Encode(map[string]any{
				"methodResponses": []any{
					[]any{"Blob/upload", map[string]any{
						"created": map[string]any{
							"blob1": map[string]any{"id": "blob-123"},
						},
					}, "0"},
				},
			})
		case 2:
			// SieveScript/query — no existing scripts
			assert.Equal(t, "SieveScript/query", method)
			json.NewEncoder(w).Encode(map[string]any{
				"methodResponses": []any{
					[]any{"SieveScript/query", map[string]any{"ids": []any{}}, "0"},
				},
			})
		case 3:
			// SieveScript/set — create
			assert.Equal(t, "SieveScript/set", method)
			args := call[1].(map[string]any)
			assert.Equal(t, "2j", args["accountId"])
			assert.Equal(t, "#script1", args["onSuccessActivateScript"])
			create := args["create"].(map[string]any)
			script := create["script1"].(map[string]any)
			assert.Equal(t, "test-script", script["name"])
			assert.Equal(t, "blob-123", script["blobId"])
			// isActive should NOT be present
			_, hasIsActive := script["isActive"]
			assert.False(t, hasIsActive, "isActive should not be set on create")
			json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
		default:
			t.Fatalf("unexpected call %d", callCount)
		}
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeploySieveScript(context.Background(), srv.URL, "test-token", "2j", "test-script", "redirect \"bob@gmail.com\";")
	require.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestJMAPClient_DeploySieveScript_Update(t *testing.T) {
	callCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		method := call[0].(string)

		switch callCount {
		case 1:
			// Blob/upload
			json.NewEncoder(w).Encode(map[string]any{
				"methodResponses": []any{
					[]any{"Blob/upload", map[string]any{
						"created": map[string]any{
							"blob1": map[string]any{"id": "blob-456"},
						},
					}, "0"},
				},
			})
		case 2:
			// SieveScript/query — existing script found
			json.NewEncoder(w).Encode(map[string]any{
				"methodResponses": []any{
					[]any{"SieveScript/query", map[string]any{"ids": []any{"existing-script-id"}}, "0"},
				},
			})
		case 3:
			// SieveScript/set — update
			assert.Equal(t, "SieveScript/set", method)
			args := call[1].(map[string]any)
			assert.Equal(t, "existing-script-id", args["onSuccessActivateScript"])
			update := args["update"].(map[string]any)
			script := update["existing-script-id"].(map[string]any)
			assert.Equal(t, "blob-456", script["blobId"])
			json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
		}
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeploySieveScript(context.Background(), srv.URL, "test-token", "2j", "test-script", "redirect \"bob@gmail.com\";")
	require.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestJMAPClient_DeploySieveScript_UploadFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("upload failed"))
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeploySieveScript(context.Background(), srv.URL, "test-token", "2j", "test-script", "content")
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

		// Destroy response — verify deactivate + destroy.
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		assert.Equal(t, "SieveScript/set", call[0])
		args := call[1].(map[string]any)
		assert.Nil(t, args["onSuccessActivateScript"])
		assert.Equal(t, []any{"script-1"}, args["destroy"])
		json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.DeleteSieveScript(context.Background(), srv.URL, "test-token", "2j", "test-script")
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
	err := client.DeleteSieveScript(context.Background(), srv.URL, "test-token", "2j", "test-script")
	require.NoError(t, err)
}

func TestJMAPClient_SetVacationResponse_Enable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/jmap", r.URL.Path)

		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		assert.Equal(t, "VacationResponse/set", call[0])
		args := call[1].(map[string]any)
		assert.Equal(t, "2j", args["accountId"])
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
	err := client.SetVacationResponse(context.Background(), srv.URL, "test-token", "2j", &VacationParams{
		Subject: "Out of office",
		Body:    "I'm on vacation.",
		Enabled: true,
	})
	require.NoError(t, err)
}

func TestJMAPClient_SetVacationResponse_EnableWithDates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		args := call[1].(map[string]any)
		update := args["update"].(map[string]any)
		singleton := update["singleton"].(map[string]any)
		assert.Equal(t, "2026-01-01T00:00:00Z", singleton["fromDate"])
		assert.Equal(t, "2026-01-15T00:00:00Z", singleton["toDate"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
	}))
	defer srv.Close()

	start := "2026-01-01T00:00:00Z"
	end := "2026-01-15T00:00:00Z"
	client := NewJMAPClient()
	err := client.SetVacationResponse(context.Background(), srv.URL, "test-token", "2j", &VacationParams{
		Subject:   "Out of office",
		Body:      "I'm on vacation.",
		Enabled:   true,
		StartDate: &start,
		EndDate:   &end,
	})
	require.NoError(t, err)
}

func TestJMAPClient_SetVacationResponse_NilDatesOmitted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		calls := req["methodCalls"].([]any)
		call := calls[0].([]any)
		args := call[1].(map[string]any)
		update := args["update"].(map[string]any)
		singleton := update["singleton"].(map[string]any)
		// Nil dates should be omitted entirely, not set to null.
		_, hasFrom := singleton["fromDate"]
		_, hasTo := singleton["toDate"]
		assert.False(t, hasFrom, "fromDate should be omitted when nil")
		assert.False(t, hasTo, "toDate should be omitted when nil")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"methodResponses": []any{}})
	}))
	defer srv.Close()

	client := NewJMAPClient()
	err := client.SetVacationResponse(context.Background(), srv.URL, "test-token", "2j", &VacationParams{
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
	err := client.SetVacationResponse(context.Background(), srv.URL, "test-token", "2j", nil)
	require.NoError(t, err)
}

func TestJMAPClient_SieveCapabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		json.NewDecoder(r.Body).Decode(&req)
		using := req["using"].([]any)
		// Verify urn:ietf:params:jmap:sieve is used (not managesieve).
		found := false
		for _, u := range using {
			assert.NotEqual(t, "urn:ietf:params:jmap:managesieve", u, "should not use managesieve URN")
			if u == "urn:ietf:params:jmap:sieve" {
				found = true
			}
		}
		assert.True(t, found, "should use urn:ietf:params:jmap:sieve")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"methodResponses": []any{
				[]any{"SieveScript/query", map[string]any{"ids": []any{}}, "0"},
			},
		})
	}))
	defer srv.Close()

	client := NewJMAPClient()
	// DeleteSieveScript makes a query request first — good for checking capabilities.
	_ = client.DeleteSieveScript(context.Background(), srv.URL, "test-token", "2j", "test")
}
