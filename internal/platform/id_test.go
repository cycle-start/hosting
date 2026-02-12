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

func TestNewShortID_Format(t *testing.T) {
	id := NewShortID()
	assert.Len(t, id, 10)
	assert.Regexp(t, regexp.MustCompile(`^[a-z0-9]{10}$`), id)
}

func TestNewShortID_ReturnsUniqueValues(t *testing.T) {
	seen := make(map[string]bool, 100)
	for i := 0; i < 100; i++ {
		id := NewShortID()
		assert.False(t, seen[id], "duplicate short ID generated: %s", id)
		seen[id] = true
	}
	assert.Len(t, seen, 100)
}
