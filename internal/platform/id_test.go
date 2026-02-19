package platform

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewID_ReturnsValidUUIDString(t *testing.T) {
	id := NewID()
	assert.NotEmpty(t, id)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, id)
}

func TestNewID_ReturnsUniqueValues(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := NewID()
		assert.False(t, seen[id], "duplicate ID generated: %s", id)
		seen[id] = true
	}
	assert.Len(t, seen, 100)
}

func TestNewName_Format(t *testing.T) {
	tests := []struct {
		prefix   string
		expected *regexp.Regexp
	}{
		{"t", regexp.MustCompile(`^t[a-z0-9]{10}$`)},
		{"db", regexp.MustCompile(`^db[a-z0-9]{10}$`)},
		{"kv", regexp.MustCompile(`^kv[a-z0-9]{10}$`)},
		{"s3", regexp.MustCompile(`^s3[a-z0-9]{10}$`)},
		{"w", regexp.MustCompile(`^w[a-z0-9]{10}$`)},
		{"cj", regexp.MustCompile(`^cj[a-z0-9]{10}$`)},
		{"d", regexp.MustCompile(`^d[a-z0-9]{10}$`)},
	}
	for _, tt := range tests {
		name := NewName(tt.prefix)
		assert.Regexp(t, tt.expected, name, "prefix=%s", tt.prefix)
	}
}

func TestNewName_ReturnsUniqueValues(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		name := NewName("t")
		assert.False(t, seen[name], "duplicate name generated: %s", name)
		seen[name] = true
	}
	assert.Len(t, seen, 100)
}
