package workflow

import (
	"testing"

	"github.com/edvin/hosting/internal/model"
)

func TestResolveStorageVolumes(t *testing.T) {
	tests := []struct {
		name          string
		shard         model.Shard
		dockerNetwork string
		cephMountBase string
		want          []string
	}{
		{
			name:          "web shard with docker network uses named volumes",
			shard:         model.Shard{Name: "web-1", Role: model.ShardRoleWeb},
			dockerNetwork: "hosting_default",
			want: []string{
				"hosting-web-1-storage:/var/www/storage",
				"hosting-web-1-homes:/home",
			},
		},
		{
			name:          "web shard with ceph mount base uses bind mounts",
			shard:         model.Shard{Name: "web-1", Role: model.ShardRoleWeb},
			cephMountBase: "/mnt/ceph/hosting",
			want: []string{
				"/mnt/ceph/hosting/web-1/storage:/var/www/storage",
				"/mnt/ceph/hosting/web-1/homes:/home",
			},
		},
		{
			name:          "web shard with both prefers docker network",
			shard:         model.Shard{Name: "web-1", Role: model.ShardRoleWeb},
			dockerNetwork: "hosting_default",
			cephMountBase: "/mnt/ceph/hosting",
			want: []string{
				"hosting-web-1-storage:/var/www/storage",
				"hosting-web-1-homes:/home",
			},
		},
		{
			name:  "web shard with neither returns nil",
			shard: model.Shard{Name: "web-1", Role: model.ShardRoleWeb},
			want:  nil,
		},
		{
			name:          "database shard returns nil",
			shard:         model.Shard{Name: "db-1", Role: model.ShardRoleDatabase},
			dockerNetwork: "hosting_default",
			want:          nil,
		},
		{
			name:          "dns shard returns nil",
			shard:         model.Shard{Name: "dns-1", Role: model.ShardRoleDNS},
			dockerNetwork: "hosting_default",
			want:          nil,
		},
		{
			name:          "valkey shard returns nil",
			shard:         model.Shard{Name: "valkey-1", Role: model.ShardRoleValkey},
			cephMountBase: "/mnt/ceph/hosting",
			want:          nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveStorageVolumes(tt.shard, tt.dockerNetwork, tt.cephMountBase)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d volumes, want %d: %v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("volume[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
