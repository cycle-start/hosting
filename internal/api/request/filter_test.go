package request

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseListParams_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants", nil)
	p := ParseListParams(r, "created_at")
	assert.Equal(t, DefaultLimit, p.Limit)
	assert.Empty(t, p.Cursor)
	assert.Empty(t, p.Search)
	assert.Empty(t, p.Status)
	assert.Equal(t, "created_at", p.Sort)
	assert.Equal(t, "desc", p.Order)
}

func TestParseListParams_AllParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?limit=25&cursor=abc123&search=my-tenant&status=active&sort=name&order=asc", nil)
	p := ParseListParams(r, "created_at")
	assert.Equal(t, 25, p.Limit)
	assert.Equal(t, "abc123", p.Cursor)
	assert.Equal(t, "my-tenant", p.Search)
	assert.Equal(t, "active", p.Status)
	assert.Equal(t, "name", p.Sort)
	assert.Equal(t, "asc", p.Order)
}

func TestParseListParams_DefaultSort(t *testing.T) {
	r := httptest.NewRequest("GET", "/nodes", nil)
	p := ParseListParams(r, "hostname")
	assert.Equal(t, "hostname", p.Sort)
}

func TestParseListParams_InvalidOrderFallsBack(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?order=invalid", nil)
	p := ParseListParams(r, "created_at")
	assert.Equal(t, "desc", p.Order)
}

func TestParseListParams_AscOrder(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?order=asc", nil)
	p := ParseListParams(r, "created_at")
	assert.Equal(t, "asc", p.Order)
}

func TestParseListParams_DescOrder(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?order=desc", nil)
	p := ParseListParams(r, "created_at")
	assert.Equal(t, "desc", p.Order)
}

func TestParseListParams_SearchOnly(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?search=foo", nil)
	p := ParseListParams(r, "created_at")
	assert.Equal(t, "foo", p.Search)
	assert.Empty(t, p.Status)
}

func TestParseListParams_StatusOnly(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?status=suspended", nil)
	p := ParseListParams(r, "created_at")
	assert.Empty(t, p.Search)
	assert.Equal(t, "suspended", p.Status)
}

func TestParseListParams_LimitClamped(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?limit=500", nil)
	p := ParseListParams(r, "created_at")
	assert.Equal(t, MaxLimit, p.Limit)
}

func TestStringOr(t *testing.T) {
	assert.Equal(t, "hello", stringOr("hello", "world"))
	assert.Equal(t, "world", stringOr("", "world"))
}
