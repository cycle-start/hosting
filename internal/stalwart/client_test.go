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

// ---------- CreateDomain ----------

func TestClient_CreateDomain_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/principal", r.URL.Path)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "expected basic auth")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "test-token", pass)

		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, "domain", payload["type"])
		assert.Equal(t, "example.com", payload["name"])

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":1}`))
	}))
	defer srv.Close()

	client := NewClient()
	err := client.CreateDomain(context.Background(), srv.URL, "test-token", "example.com")
	require.NoError(t, err)
}

func TestClient_CreateDomain_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	client := NewClient()
	err := client.CreateDomain(context.Background(), srv.URL, "test-token", "example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 500")
	assert.Contains(t, err.Error(), "internal error")
}

// ---------- DeleteDomain ----------

func TestClient_DeleteDomain_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/principal/example.com", r.URL.Path)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "expected basic auth")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "test-token", pass)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient()
	err := client.DeleteDomain(context.Background(), srv.URL, "test-token", "example.com")
	require.NoError(t, err)
}

func TestClient_DeleteDomain_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := NewClient()
	err := client.DeleteDomain(context.Background(), srv.URL, "test-token", "example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
	assert.Contains(t, err.Error(), "not found")
}

// ---------- CreateAccount ----------

func TestClient_CreateAccount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/principal", r.URL.Path)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "expected basic auth")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "test-token", pass)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var payload map[string]any
		err := json.NewDecoder(r.Body).Decode(&payload)
		require.NoError(t, err)
		assert.Equal(t, "individual", payload["type"])
		assert.Equal(t, "user@example.com", payload["name"])
		assert.Equal(t, "Test User", payload["description"])
		assert.Equal(t, float64(1073741824), payload["quota"])

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient()
	err := client.CreateAccount(context.Background(), srv.URL, "test-token", CreateAccountParams{
		Address:     "user@example.com",
		DisplayName: "Test User",
		QuotaBytes:  1073741824,
		Password:    "secret123",
	})
	require.NoError(t, err)
}

func TestClient_CreateAccount_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte("account exists"))
	}))
	defer srv.Close()

	client := NewClient()
	err := client.CreateAccount(context.Background(), srv.URL, "test-token", CreateAccountParams{
		Address:     "user@example.com",
		DisplayName: "Test User",
		QuotaBytes:  1073741824,
		Password:    "secret123",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 409")
	assert.Contains(t, err.Error(), "account exists")
}

// ---------- DeleteAccount ----------

func TestClient_DeleteAccount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method)
		assert.Equal(t, "/api/principal/user@example.com", r.URL.Path)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "expected basic auth")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "test-token", pass)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient()
	err := client.DeleteAccount(context.Background(), srv.URL, "test-token", "user@example.com")
	require.NoError(t, err)
}

func TestClient_DeleteAccount_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := NewClient()
	err := client.DeleteAccount(context.Background(), srv.URL, "test-token", "user@example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
	assert.Contains(t, err.Error(), "not found")
}

// ---------- UpdateAccount ----------

func TestClient_UpdateAccount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPatch, r.Method)
		assert.Equal(t, "/api/principal/user@example.com", r.URL.Path)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "expected basic auth")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "test-token", pass)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var ops []PatchOp
		err := json.NewDecoder(r.Body).Decode(&ops)
		require.NoError(t, err)
		require.Len(t, ops, 1)
		assert.Equal(t, "addItem", ops[0].Action)
		assert.Equal(t, "emails", ops[0].Field)
		assert.Equal(t, "alias@example.com", ops[0].Value)

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient()
	err := client.UpdateAccount(context.Background(), srv.URL, "test-token", "user@example.com", []PatchOp{
		{Action: "addItem", Field: "emails", Value: "alias@example.com"},
	})
	require.NoError(t, err)
}

func TestClient_UpdateAccount_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("principal not found"))
	}))
	defer srv.Close()

	client := NewClient()
	err := client.UpdateAccount(context.Background(), srv.URL, "test-token", "user@example.com", []PatchOp{
		{Action: "addItem", Field: "emails", Value: "alias@example.com"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status 404")
	assert.Contains(t, err.Error(), "principal not found")
}

// ---------- GetAccount ----------

func TestClient_GetAccount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/principal/user@example.com", r.URL.Path)
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok, "expected basic auth")
		assert.Equal(t, "admin", user)
		assert.Equal(t, "test-token", pass)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"name":        "user@example.com",
			"description": "Test User",
			"quota":       1073741824,
		})
	}))
	defer srv.Close()

	client := NewClient()
	acct, err := client.GetAccount(context.Background(), srv.URL, "test-token", "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, acct)
	assert.Equal(t, "user@example.com", acct.Address)
	assert.Equal(t, "Test User", acct.DisplayName)
	assert.Equal(t, int64(1073741824), acct.QuotaBytes)
}

func TestClient_GetAccount_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := NewClient()
	acct, err := client.GetAccount(context.Background(), srv.URL, "test-token", "user@example.com")
	require.Error(t, err)
	assert.Nil(t, acct)
	assert.Contains(t, err.Error(), "status 404")
	assert.Contains(t, err.Error(), "not found")
}
