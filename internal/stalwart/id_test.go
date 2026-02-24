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
		{0, "a"},
		{1, "b"},
		{26, "7"},
		{31, "3"},
		{32, "ba"},
		{67, "cd"},
		{82, "cs"},
		{101, "df"},
		{1000, "3i"},
		{4294967295, "d333333"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, EncodePrincipalID(tt.id))
		})
	}
}
