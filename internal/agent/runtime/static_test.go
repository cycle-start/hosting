package runtime

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
)

func TestStatic_AllMethodsReturnNil(t *testing.T) {
	s := NewStatic(zerolog.Nop())

	webroot := &agentv1.WebrootInfo{
		TenantName: "testtenant",
		Name:       "testwebroot",
		Runtime:    "static",
	}

	ctx := context.Background()

	assert.NoError(t, s.Configure(ctx, webroot))
	assert.NoError(t, s.Start(ctx, webroot))
	assert.NoError(t, s.Stop(ctx, webroot))
	assert.NoError(t, s.Reload(ctx, webroot))
	assert.NoError(t, s.Remove(ctx, webroot))
}

func TestStatic_ImplementsManagerInterface(t *testing.T) {
	s := NewStatic(zerolog.Nop())

	// Verify Static implements the Manager interface at compile time.
	var _ Manager = s
}

func TestStatic_NilWebroot(t *testing.T) {
	s := NewStatic(zerolog.Nop())
	ctx := context.Background()

	// Static methods should handle nil webroot gracefully (they just log fields).
	// The protobuf Get* methods are nil-safe.
	assert.NoError(t, s.Configure(ctx, nil))
	assert.NoError(t, s.Start(ctx, nil))
	assert.NoError(t, s.Stop(ctx, nil))
	assert.NoError(t, s.Reload(ctx, nil))
	assert.NoError(t, s.Remove(ctx, nil))
}

func TestStatic_EmptyWebroot(t *testing.T) {
	s := NewStatic(zerolog.Nop())
	ctx := context.Background()

	webroot := &agentv1.WebrootInfo{}

	assert.NoError(t, s.Configure(ctx, webroot))
	assert.NoError(t, s.Start(ctx, webroot))
	assert.NoError(t, s.Stop(ctx, webroot))
	assert.NoError(t, s.Reload(ctx, webroot))
	assert.NoError(t, s.Remove(ctx, webroot))
}
