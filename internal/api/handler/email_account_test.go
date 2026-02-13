package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newEmailAccountHandler() *EmailAccount {
	return &EmailAccount{svc: nil, services: nil}
}

// --- Nested resource validation ---

func TestEmailAccountCreate_WithNestedAliases_ValidationPasses(t *testing.T) {
	h := newEmailAccountHandler()
	rec := httptest.NewRecorder()
	fqdnID := "test-fqdn-1"
	r := newRequest(http.MethodPost, "/fqdns/"+fqdnID+"/email-accounts", map[string]any{
		"address": "admin@example.com",
		"aliases": []map[string]any{
			{"address": "postmaster@example.com"},
			{"address": "webmaster@example.com"},
		},
	})
	r = withChiURLParam(r, "fqdnID", fqdnID)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestEmailAccountCreate_WithNestedForwards_ValidationPasses(t *testing.T) {
	h := newEmailAccountHandler()
	rec := httptest.NewRecorder()
	fqdnID := "test-fqdn-1"
	r := newRequest(http.MethodPost, "/fqdns/"+fqdnID+"/email-accounts", map[string]any{
		"address": "admin@example.com",
		"forwards": []map[string]any{
			{"destination": "backup@other.com", "keep_copy": true},
			{"destination": "archive@other.com"},
		},
	})
	r = withChiURLParam(r, "fqdnID", fqdnID)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestEmailAccountCreate_WithNestedAutoReply_ValidationPasses(t *testing.T) {
	h := newEmailAccountHandler()
	rec := httptest.NewRecorder()
	fqdnID := "test-fqdn-1"
	r := newRequest(http.MethodPost, "/fqdns/"+fqdnID+"/email-accounts", map[string]any{
		"address": "admin@example.com",
		"autoreply": map[string]any{
			"subject": "Out of office",
			"body":    "I am currently away.",
			"enabled": true,
		},
	})
	r = withChiURLParam(r, "fqdnID", fqdnID)

	func() {
		defer func() { recover() }()
		h.Create(rec, r)
	}()

	assert.NotEqual(t, http.StatusBadRequest, rec.Code)
}

func TestEmailAccountCreate_WithInvalidNestedAlias_ValidationFails(t *testing.T) {
	h := newEmailAccountHandler()
	rec := httptest.NewRecorder()
	fqdnID := "test-fqdn-1"
	r := newRequest(http.MethodPost, "/fqdns/"+fqdnID+"/email-accounts", map[string]any{
		"address": "admin@example.com",
		"aliases": []map[string]any{
			{"address": "not-an-email"}, // invalid email
		},
	})
	r = withChiURLParam(r, "fqdnID", fqdnID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestEmailAccountCreate_WithInvalidNestedForward_ValidationFails(t *testing.T) {
	h := newEmailAccountHandler()
	rec := httptest.NewRecorder()
	fqdnID := "test-fqdn-1"
	r := newRequest(http.MethodPost, "/fqdns/"+fqdnID+"/email-accounts", map[string]any{
		"address": "admin@example.com",
		"forwards": []map[string]any{
			{"destination": "not-an-email"}, // invalid email
		},
	})
	r = withChiURLParam(r, "fqdnID", fqdnID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}

func TestEmailAccountCreate_WithInvalidNestedAutoReply_ValidationFails(t *testing.T) {
	h := newEmailAccountHandler()
	rec := httptest.NewRecorder()
	fqdnID := "test-fqdn-1"
	r := newRequest(http.MethodPost, "/fqdns/"+fqdnID+"/email-accounts", map[string]any{
		"address": "admin@example.com",
		"autoreply": map[string]any{
			"subject": "Out of office",
			// missing body
		},
	})
	r = withChiURLParam(r, "fqdnID", fqdnID)

	h.Create(rec, r)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	body := decodeErrorResponse(rec)
	assert.Contains(t, body["error"], "validation error")
}
