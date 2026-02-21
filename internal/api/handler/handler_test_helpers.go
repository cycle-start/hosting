package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	mw "github.com/edvin/hosting/internal/api/middleware"
	"github.com/go-chi/chi/v5"
)

// newRequest creates a new HTTP request with an optional JSON body.
func newRequest(method, target string, body any) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	r := httptest.NewRequest(method, target, &buf)
	r.Header.Set("Content-Type", "application/json")
	return r
}

// newRequestRaw creates a new HTTP request with a raw string body.
func newRequestRaw(method, target, body string) *http.Request {
	r := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// withChiURLParam adds a chi URL parameter to the request context.
func withChiURLParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// withChiURLParams adds multiple chi URL parameters to the request context.
func withChiURLParams(r *http.Request, params map[string]string) *http.Request {
	rctx := chi.NewRouteContext()
	for key, value := range params {
		rctx.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// decodeErrorResponse parses the JSON error response body into a map.
func decodeErrorResponse(rec *httptest.ResponseRecorder) map[string]string {
	var body map[string]string
	json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

// withPlatformAdmin injects a platform-admin identity (brands: ["*"]) into the request context.
func withPlatformAdmin(r *http.Request) *http.Request {
	identity := &mw.APIKeyIdentity{
		ID:     "test-admin-key",
		Scopes: []string{"*:*"},
		Brands: []string{"*"},
	}
	ctx := context.WithValue(r.Context(), mw.APIKeyIdentityKey, identity)
	return r.WithContext(ctx)
}

const validID = "test-id-1"
const validID2 = "test-id-2"
