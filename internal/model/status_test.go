package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusConstants(t *testing.T) {
	assert.Equal(t, "pending", StatusPending)
	assert.Equal(t, "provisioning", StatusProvisioning)
	assert.Equal(t, "active", StatusActive)
	assert.Equal(t, "failed", StatusFailed)
	assert.Equal(t, "suspended", StatusSuspended)
	assert.Equal(t, "deleting", StatusDeleting)
	assert.Equal(t, "deleted", StatusDeleted)
}
