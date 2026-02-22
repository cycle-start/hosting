package stalwart

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodePrincipalID(t *testing.T) {
	tests := []struct {
		id   uint32
		want string
	}{
		{0, "0"},
		{1, "1"},
		{26, "t"},
		{31, "z"},
		{32, "10"},
		{82, "2j"},
		{1000, "z8"},
		{4294967295, "3zzzzzz"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, EncodePrincipalID(tt.id))
		})
	}
}
