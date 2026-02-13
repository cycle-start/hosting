package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDashboardStats_NilService(t *testing.T) {
	// With nil service, calling Stats will panic (nil pointer dereference).
	// This confirms the handler delegates to the service correctly.
	h := NewDashboard(nil)
	rec := httptest.NewRecorder()
	r := newRequest(http.MethodGet, "/dashboard/stats", nil)

	panicked := false
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		h.Stats(rec, r)
	}()

	// Should panic because the service is nil and we try to call it
	assert.True(t, panicked, "expected panic due to nil service")
}

func TestNewDashboard(t *testing.T) {
	h := NewDashboard(nil)
	assert.NotNil(t, h)
}
