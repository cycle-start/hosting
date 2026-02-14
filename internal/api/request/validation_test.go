package request

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireID_Valid(t *testing.T) {
	result, err := RequireID("550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", result)
}

func TestRequireID_ShortID(t *testing.T) {
	result, err := RequireID("abc1234xyz")
	require.NoError(t, err)
	assert.Equal(t, "abc1234xyz", result)
}

func TestRequireID_Empty(t *testing.T) {
	_, err := RequireID("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required ID")
}

// testDecodePayload is a helper struct used only for testing Decode.
type testDecodePayload struct {
	Name  string `json:"name" validate:"required"`
	Email string `json:"email" validate:"required,email"`
}

func TestDecode_ValidJSON(t *testing.T) {
	body := `{"name":"alice","email":"alice@example.com"}`
	r, err := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	require.NoError(t, err)

	var payload testDecodePayload
	err = Decode(r, &payload)
	require.NoError(t, err)
	assert.Equal(t, "alice", payload.Name)
	assert.Equal(t, "alice@example.com", payload.Email)
}

func TestDecode_InvalidJSON(t *testing.T) {
	body := `{not valid json}`
	r, err := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	require.NoError(t, err)

	var payload testDecodePayload
	err = Decode(r, &payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid JSON")
}

func TestDecode_ValidationFails(t *testing.T) {
	// Missing the required "name" field.
	body := `{"email":"alice@example.com"}`
	r, err := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	require.NoError(t, err)

	var payload testDecodePayload
	err = Decode(r, &payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation error")
}

func TestSlugValidation_Valid(t *testing.T) {
	validSlugs := []string{"my-site", "test123", "a", "abc-def-123", "z0"}
	for _, slug := range validSlugs {
		t.Run(slug, func(t *testing.T) {
			assert.True(t, nameRegex.MatchString(slug), "expected slug %q to be valid", slug)
		})
	}
}

func TestSlugValidation_Invalid(t *testing.T) {
	invalidSlugs := []string{
		"My Site",       // spaces and uppercase
		"test@123",      // special character
		"",              // empty
		strings.Repeat("a", 64), // too long (max 63 chars)
		"1starts-digit", // must start with lowercase letter
		"-leading-dash", // must start with lowercase letter
	}
	for _, slug := range invalidSlugs {
		t.Run(slug, func(t *testing.T) {
			assert.False(t, nameRegex.MatchString(slug), "expected slug %q to be invalid", slug)
		})
	}
}

func TestMySQLNameValidation_Valid(t *testing.T) {
	validNames := []string{"mydb", "test123", "a", "my_database", "db_01", "z0"}
	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			assert.True(t, mysqlNameRegex.MatchString(name), "expected mysql_name %q to be valid", name)
		})
	}
}

func TestMySQLNameValidation_Invalid(t *testing.T) {
	invalidNames := []string{
		"my-database",   // hyphens not allowed
		"My_DB",         // uppercase not allowed
		"test@123",      // special character
		"",              // empty
		strings.Repeat("a", 64), // too long (max 63 chars)
		"1starts_digit", // must start with lowercase letter
		"_leading",      // must start with lowercase letter
		"-leading",      // must start with lowercase letter
		"has-hyphen",    // hyphens not allowed
	}
	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			assert.False(t, mysqlNameRegex.MatchString(name), "expected mysql_name %q to be invalid", name)
		})
	}
}
