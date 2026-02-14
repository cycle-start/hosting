package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newDatabaseUserHandler() *DatabaseUser {
	return NewDatabaseUser(nil)
}

// --- ListByDatabase ---

func TestDatabaseUserListByDatabase_EmptyID(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/databases//users", nil)
	r = withChiURLParam(r, "databaseID", "")

	h.ListByDatabase(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Create ---

func TestDatabaseUserCreate_EmptyDatabaseID(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases//users", map[string]any{
		"username":   "myuser",
		"password":   "secure-password-123",
		"privileges": []string{"ALL"},
	})
	r = withChiURLParam(r, "databaseID", "")

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestDatabaseUserCreate_InvalidJSON(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/databases/"+validID+"/users", "{bad json")
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestDatabaseUserCreate_EmptyBody(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/databases/"+validID+"/users", "")
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDatabaseUserCreate_MissingRequiredFields(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/users", map[string]any{})
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseUserCreate_MissingUsername(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/users", map[string]any{
		"password":   "secure-password-123",
		"privileges": []string{"ALL"},
	})
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseUserCreate_MissingPassword(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/users", map[string]any{
		"username":   "myuser",
		"privileges": []string{"ALL"},
	})
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseUserCreate_MissingPrivileges(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/users", map[string]any{
		"username": "myuser",
		"password": "secure-password-123",
	})
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseUserCreate_EmptyPrivileges(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/users", map[string]any{
		"username":   "myuser",
		"password":   "secure-password-123",
		"privileges": []string{},
	})
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseUserCreate_PasswordTooShort(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPost, "/databases/"+validID+"/users", map[string]any{
		"username":   "myuser",
		"password":   "short",
		"privileges": []string{"ALL"},
	})
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseUserCreate_InvalidSlugUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
	}{
		{"uppercase", "MyUser"},
		{"spaces", "my user"},
		{"special chars", "user@name"},
		{"starts with digit", "1user"},
		{"hyphens", "my-user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newDatabaseUserHandler()
			rec := httptest.NewRecorder()
			r := newRequest(http.MethodPost, "/databases/"+validID+"/users", map[string]any{
				"username":   tt.username,
				"password":   "secure-password-123",
				"privileges": []string{"ALL"},
			})
			r = withChiURLParam(r, "databaseID", validID)

			h.Create(rec, r)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestDatabaseUserCreate_ValidBody(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	dbid := "test-database-1"
	r := newRequest(http.MethodPost, "/databases/"+dbid+"/users", map[string]any{
		"username":   "myuser",
		"password":   "secure-password-123",
		"privileges": []string{"ALL", "SELECT"},
	})
	r = withChiURLParam(r, "databaseID", dbid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestDatabaseUserCreate_MinimumPasswordLength(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	dbid := "test-database-2"
	r := newRequest(http.MethodPost, "/databases/"+dbid+"/users", map[string]any{
		"username":   "myuser",
		"password":   "12345678", // exactly 8 characters (min)
		"privileges": []string{"ALL"},
	})
	r = withChiURLParam(r, "databaseID", dbid)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

// --- Get ---

func TestDatabaseUserGet_EmptyID(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/database-users/", nil)
	r = withChiURLParam(r, "id", "")

	h.Get(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Update ---

func TestDatabaseUserUpdate_EmptyID(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/database-users/", map[string]any{
		"password": "new-password-123",
	})
	r = withChiURLParam(r, "id", "")

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

func TestDatabaseUserUpdate_InvalidJSON(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/database-users/"+validID, "{bad json")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "invalid JSON")
}

func TestDatabaseUserUpdate_EmptyBody(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPut, "/database-users/"+validID, "")
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDatabaseUserUpdate_PasswordTooShort(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/database-users/"+validID, map[string]any{
		"password": "short",
	})
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestDatabaseUserUpdate_EmptyPrivileges(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodPut, "/database-users/"+validID, map[string]any{
		"privileges": []string{},
	})
	r = withChiURLParam(r, "id", validID)

	h.Update(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

// --- Delete ---

func TestDatabaseUserDelete_EmptyID(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodDelete, "/database-users/", nil)
	r = withChiURLParam(r, "id", "")

	h.Delete(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "missing required ID")
}

// --- Error response format ---

func TestDatabaseUserCreate_ErrorResponseFormat(t *testing.T) {
	h := newDatabaseUserHandler()
	rec := httptest.NewRecorder()
	r := newRequestRaw(http.MethodPost, "/databases/"+validID+"/users", "{bad")
	r = withChiURLParam(r, "databaseID", validID)

	h.Create(rec, r)

	var body map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &body)
	require.NoError(t, err)
	_, hasError := body["error"]
	assert.True(t, hasError)
}
