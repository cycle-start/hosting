package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCheckMount_NonexistentPath(t *testing.T) {
	err := checkMount("/nonexistent/path/that/does/not/exist")
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
	assert.Contains(t, st.Message(), "CephFS not mounted")
}

func TestCheckMount_NotCephFS(t *testing.T) {
	// Use a known-mounted non-CephFS path (like /tmp).
	err := checkMount("/tmp")
	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
	assert.Contains(t, st.Message(), "unexpected filesystem")
}
