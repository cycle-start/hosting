package middleware

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractResource_SimplePath(t *testing.T) {
	resType, resID := extractResource("/api/v1/tenants")
	assert.NotNil(t, resType)
	assert.Equal(t, "tenants", *resType)
	assert.Nil(t, resID)
}

func TestExtractResource_WithID(t *testing.T) {
	resType, resID := extractResource("/api/v1/tenants/abc-123")
	assert.NotNil(t, resType)
	assert.Equal(t, "tenants", *resType)
	assert.NotNil(t, resID)
	assert.Equal(t, "abc-123", *resID)
}

func TestExtractResource_Nested(t *testing.T) {
	resType, resID := extractResource("/api/v1/tenants/abc/webroots/def")
	assert.NotNil(t, resType)
	assert.Equal(t, "webroots", *resType)
	assert.NotNil(t, resID)
	assert.Equal(t, "def", *resID)
}

func TestExtractResource_NestedNoID(t *testing.T) {
	resType, resID := extractResource("/api/v1/tenants/abc/webroots")
	assert.NotNil(t, resType)
	assert.Equal(t, "webroots", *resType)
	assert.Nil(t, resID)
}

func TestSanitizeBody(t *testing.T) {
	body := []byte(`{"name":"test","password":"secret123","key_pem":"---BEGIN---"}`)
	sanitized := sanitizeBody(body)

	var result map[string]any
	json.Unmarshal(sanitized, &result)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, "[REDACTED]", result["password"])
	assert.Equal(t, "[REDACTED]", result["key_pem"])
}
