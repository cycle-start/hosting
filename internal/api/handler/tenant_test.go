package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTenantHandler() *Tenant {
	return &Tenant{svc: nil, services: nil}
}

// --- Create ---

func TestTenantCreate_InvalidJSON(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants", "{bad json")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestTenantCreate_EmptyBody(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestTenantCreate_MissingRequiredFields(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestTenantCreate_MissingRegionID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"cluster_id": validID2,
		"shard_id":   "test-shard-1",
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestTenantCreate_MissingClusterID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"region_id": validID,
		"shard_id":  "test-shard-1",
	})

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Get ---

func TestTenantGet_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestTenantGet_MissingURLParam(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	// No chi context set, so URLParam returns ""
	r := newRequest(http.MethodGet, "/tenants/", nil)

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Update ---

func TestTenantUpdate_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/tenants/", map[string]any{
		"sftp_enabled": true,
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestTenantUpdate_InvalidJSON(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/tenants/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestTenantUpdate_EmptyBody(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/tenants/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

// --- Delete ---

func TestTenantDelete_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/tenants/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Suspend ---

func TestTenantSuspend_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//suspend", nil)
	r = withChiURLParam(r, "id", "")

	h.Suspend(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- Unsuspend ---

func TestTenantUnsuspend_EmptyID(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants//unsuspend", nil)
	r = withChiURLParam(r, "id", "")

	h.Unsuspend(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// --- JSON content-type verification ---

func TestTenantCreate_ResponseHasJSONContentType(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	// Send invalid body so we get a 400 without hitting the DB
	r := newRequestRaw(http.MethodPost, "/tenants", "not json")

	h.Create(rec, r)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestTenantGet_ResponseHasJSONContentType(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/tenants/test-id", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

// --- Various ID format tests ---

func TestTenantGet_ValidIDFormats(t *testing.T) {
	// These are all valid ID formats that should pass the RequireID check
	// but will fail at the service layer (nil service). We just verify they pass
	// the ID validation step.
	tests := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"my-tenant-1",
		"simple",
	}
	for _, id := range tests {
		t.Run(id, func(t *testing.T) {
			h := newTenantHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodGet, "/tenants/"+id, nil)
			r = withChiURLParam(r, "id", id)

			// This will panic or return 500/404 because svc is nil, but NOT 400.
			// We use recover to catch nil pointer dereference.
			func() {
				defer func() { recover() }()
				h.Get(rec, r)
			}()

			// If we got a response code, it should not be 400
			if rec.Code != 0 && rec.Code != 200 {
				assert.NotEqual(t, http.StatusBadRequest, rec.Code,
					"valid ID %s should not produce 400", id)
			}
		})
	}
}

func TestTenantGet_InvalidIDFormats(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"empty string", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTenantHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodGet, "/tenants/test-id", nil)
			r = withChiURLParam(r, "id", tt.id)

			h.Get(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

// --- Create body validation permutations ---

func TestTenantCreate_ValidBodyParsing(t *testing.T) {
	// Verify that a well-formed body gets past the validation/decode step.
	// It will fail at the service layer (nil svc) rather than at validation.
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"region_id":    "test-region-1",
		"cluster_id":   "test-cluster-1",
		"shard_id":     "test-shard-1",
		"sftp_enabled": true,
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	// Should NOT be 400 (validation passed)
	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_OptionalSFTPEnabled(t *testing.T) {
	// sftp_enabled is optional, so body without it should pass validation.
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"region_id":  "test-region-1",
		"cluster_id": "test-cluster-1",
		"shard_id":   "test-shard-1",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_ExtraFieldsIgnored(t *testing.T) {
	// Extra fields in JSON should not cause validation errors.
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/tenants", map[string]any{
		"region_id":   "test-region-1",
		"cluster_id":  "test-cluster-1",
		"shard_id":    "test-shard-1",
		"extra_field": "should be ignored",
	})

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Nested resource validation ---

func validTenantBody() map[string]any {
	return map[string]any{
		"region_id":  "test-region-1",
		"cluster_id": "test-cluster-1",
		"shard_id":   "test-shard-1",
	}
}

func TestTenantCreate_WithNestedZones_ValidationPasses(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["zones"] = []map[string]any{
		{"name": "example.com"},
		{"name": "example.org"},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_WithNestedWebroots_ValidationPasses(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["webroots"] = []map[string]any{
		{
			"name":            "my-site",
			"runtime":         "php",
			"runtime_version": "8.5",
			"fqdns": []map[string]any{
				{"fqdn": "example.com"},
			},
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_WithNestedDatabases_ValidationPasses(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["databases"] = []map[string]any{
		{
			"name":     "mydb",
			"shard_id": "test-db-shard",
			"users": []map[string]any{
				{
					"username":   "admin",
					"password":   "securepassword123",
					"privileges": []string{"ALL"},
				},
			},
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_WithNestedValkeyInstances_ValidationPasses(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["valkey_instances"] = []map[string]any{
		{
			"name":     "my-cache",
			"shard_id": "test-valkey-shard",
			"users": []map[string]any{
				{
					"username":   "cacheuser",
					"password":   "securepassword123",
					"privileges": []string{"allcommands"},
				},
			},
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_WithNestedSFTPKeys_ValidationPasses(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["sftp_keys"] = []map[string]any{
		{
			"name":       "my-key",
			"public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGKCwmDZb5JjFMYnbPPM6MvxMCEjMltcGacM4AiSuKiP test@localhost",
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_WithEmptyNestedArrays_ValidationPasses(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["zones"] = []map[string]any{}
	body["webroots"] = []map[string]any{}
	body["databases"] = []map[string]any{}
	body["valkey_instances"] = []map[string]any{}
	body["sftp_keys"] = []map[string]any{}
	r := newRequest(http.MethodPost, "/tenants", body)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestTenantCreate_WithInvalidNestedZone_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["zones"] = []map[string]any{
		{}, // missing name
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_WithInvalidNestedWebroot_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["webroots"] = []map[string]any{
		{
			"name":            "my-site",
			"runtime_version": "8.5",
			// missing runtime
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_WithInvalidNestedDatabase_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["databases"] = []map[string]any{
		{
			"name": "mydb",
			// missing shard_id
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_WithInvalidNestedDatabaseUser_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["databases"] = []map[string]any{
		{
			"name":     "mydb",
			"shard_id": "test-shard",
			"users": []map[string]any{
				{
					"username":   "admin",
					"password":   "short", // too short, min=8
					"privileges": []string{"ALL"},
				},
			},
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_WithInvalidNestedValkeyUser_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["valkey_instances"] = []map[string]any{
		{
			"name":     "my-cache",
			"shard_id": "test-shard",
			"users": []map[string]any{
				{
					"username": "cacheuser",
					"password": "securepassword123",
					// missing privileges
				},
			},
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_WithInvalidNestedSFTPKey_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["sftp_keys"] = []map[string]any{
		{
			"name": "my-key",
			// missing public_key
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_WithInvalidNestedFQDN_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["webroots"] = []map[string]any{
		{
			"name":            "my-site",
			"runtime":         "php",
			"runtime_version": "8.5",
			"fqdns": []map[string]any{
				{}, // missing fqdn
			},
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_WithInvalidNestedEmailAccount_ValidationFails(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["webroots"] = []map[string]any{
		{
			"name":            "my-site",
			"runtime":         "php",
			"runtime_version": "8.5",
			"fqdns": []map[string]any{
				{
					"fqdn": "example.com",
					"email_accounts": []map[string]any{
						{"address": "not-an-email"}, // invalid email
					},
				},
			},
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	errBody := decodeErrorResponse(rec)
	assert.Contains(t, errBody["error"], "validation error")
}

func TestTenantCreate_FullNested_ValidationPasses(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	body := validTenantBody()
	body["zones"] = []map[string]any{
		{"name": "example.com"},
	}
	body["webroots"] = []map[string]any{
		{
			"name":            "my-site",
			"runtime":         "php",
			"runtime_version": "8.5",
			"public_folder":   "public",
			"fqdns": []map[string]any{
				{
					"fqdn":        "example.com",
					"ssl_enabled": true,
					"email_accounts": []map[string]any{
						{
							"address":      "admin@example.com",
							"display_name": "Admin",
							"quota_bytes":  1073741824,
							"aliases": []map[string]any{
								{"address": "postmaster@example.com"},
							},
							"forwards": []map[string]any{
								{"destination": "backup@other.com", "keep_copy": true},
							},
							"autoreply": map[string]any{
								"subject": "Out of office",
								"body":    "I am away",
								"enabled": true,
							},
						},
					},
				},
			},
		},
	}
	body["databases"] = []map[string]any{
		{
			"name":     "mydb",
			"shard_id": "test-db-shard",
			"users": []map[string]any{
				{
					"username":   "admin",
					"password":   "securepassword123",
					"privileges": []string{"ALL"},
				},
			},
		},
	}
	body["valkey_instances"] = []map[string]any{
		{
			"name":          "my-cache",
			"shard_id":      "test-valkey-shard",
			"max_memory_mb": 128,
			"users": []map[string]any{
				{
					"username":    "cacheuser",
					"password":    "securepassword123",
					"privileges":  []string{"allcommands"},
					"key_pattern": "~app:*",
				},
			},
		},
	}
	body["sftp_keys"] = []map[string]any{
		{
			"name":       "deploy-key",
			"public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGKCwmDZb5JjFMYnbPPM6MvxMCEjMltcGacM4AiSuKiP test@localhost",
		},
	}
	r := newRequest(http.MethodPost, "/tenants", body)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Error response format ---

func TestTenantCreate_ErrorResponseFormat(t *testing.T) {
	h := newTenantHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/tenants", "{bad")

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)

	// Error response should have an "error" key
	_, hasError := body["error"]
	assert.True(t, hasError, "error response should contain 'error' key")
}
