package runtime

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestStatic_AllMethodsReturnNil(t *testing.T) {
	s := NewStatic(zerolog.Nop())

	webroot := &WebrootInfo{
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

func TestStatic_EmptyWebroot(t *testing.T) {
	s := NewStatic(zerolog.Nop())
	ctx := context.Background()

	webroot := &WebrootInfo{}

	assert.NoError(t, s.Configure(ctx, webroot))
	assert.NoError(t, s.Start(ctx, webroot))
	assert.NoError(t, s.Stop(ctx, webroot))
	assert.NoError(t, s.Reload(ctx, webroot))
	assert.NoError(t, s.Remove(ctx, webroot))
}
