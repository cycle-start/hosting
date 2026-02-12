package request

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants", nil)
	p := ParsePagination(r)
	assert.Equal(t, DefaultLimit, p.Limit)
	assert.Empty(t, p.Cursor)
}

func TestParsePagination_CustomValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?limit=25&cursor=abc123", nil)
	p := ParsePagination(r)
	assert.Equal(t, 25, p.Limit)
	assert.Equal(t, "abc123", p.Cursor)
}

func TestParsePagination_ExceedsMax(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?limit=500", nil)
	p := ParsePagination(r)
	assert.Equal(t, MaxLimit, p.Limit)
}

func TestParsePagination_InvalidLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?limit=abc", nil)
	p := ParsePagination(r)
	assert.Equal(t, DefaultLimit, p.Limit)
}

func TestParsePagination_ZeroLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/tenants?limit=0", nil)
	p := ParsePagination(r)
	assert.Equal(t, DefaultLimit, p.Limit)
}
