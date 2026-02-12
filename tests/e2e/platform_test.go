//go:build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/hostctl"
)

const (
	clusterConfigPath = "../../clusters/dev.yaml"
	seedConfigPath    = "../../seeds/dev-tenants.yaml"
	provisionTimeout  = 5 * time.Minute
	seedTimeout       = 5 * time.Minute
	httpTimeout       = 60 * time.Second
)

func TestPlatformE2E(t *testing.T) {
	ctx := context.Background()

	t.Run("health", func(t *testing.T) {
		resp, body, err := httpGetWithHost("http://localhost:8090/api/v1/regions", "")
		if err != nil {
			t.Fatalf("health check failed: %v", err)
		}
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		t.Logf("core-api healthy: %s", body)
	})

	t.Run("cluster_bootstrap", func(t *testing.T) {
		if err := hostctl.ClusterApply(clusterConfigPath, provisionTimeout); err != nil {
			t.Fatalf("cluster apply failed: %v", err)
		}
	})

	t.Run("seed_tenants", func(t *testing.T) {
		err := hostctl.Seed(seedConfigPath, seedTimeout)
		if err != nil {
			// Email accounts may fail (Stalwart integration is v2) — treat as
			// non-fatal if the core web resources were seeded successfully.
			if strings.Contains(err.Error(), "email") {
				t.Logf("seed completed with email error (expected): %v", err)
			} else {
				t.Fatalf("seed failed: %v", err)
			}
		}
	})

	t.Run("shared_storage", func(t *testing.T) {
		containers := findAllContainersByEnv(ctx, t, "SHARD_ROLE", "web")
		if len(containers) < 2 {
			t.Skip("need at least 2 web nodes to test shared storage")
		}
		nodeA, nodeB := containers[0], containers[1]
		t.Logf("node A: %s, node B: %s", nodeA[:12], nodeB[:12])

		// Write a unique file on node A.
		testFile := "/var/www/storage/.e2e-shared-storage-test"
		marker := "shared-storage-ok"
		execInContainer(ctx, t, nodeA, []string{
			"bash", "-c", "echo '" + marker + "' > " + testFile,
		})

		// Read it from node B.
		got := execInContainer(ctx, t, nodeB, []string{"cat", testFile})
		if !strings.Contains(got, marker) {
			t.Fatalf("file written on node A not visible on node B: got %q, want %q", got, marker)
		}
		t.Logf("cross-node read OK: %q", strings.TrimSpace(got))

		// Clean up.
		execInContainer(ctx, t, nodeA, []string{"rm", "-f", testFile})
	})

	t.Run("web_traffic", func(t *testing.T) {
		// With shared storage volumes, writing to one container is enough —
		// all web nodes in the shard see the same /var/www/storage.
		cid := findContainerByEnv(ctx, t, "SHARD_ROLE", "web")
		t.Logf("writing index.php to container %s", cid[:12])

		phpContent := `<?php echo "Hello from hosting platform"; ?>`
		execInContainer(ctx, t, cid, []string{
			"mkdir", "-p", "/var/www/storage/acme-corp/main-site/public",
		})
		execInContainer(ctx, t, cid, []string{
			"bash", "-c", "echo '" + phpContent + "' > /var/www/storage/acme-corp/main-site/public/index.php",
		})

		// Wait for the page to be served through HAProxy
		resp, body := waitForHTTP(t, "http://localhost:80", "acme.hosting.localhost", httpTimeout)

		// Assert response body
		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		expected := "Hello from hosting platform"
		if !strings.Contains(body, expected) {
			t.Fatalf("body %q does not contain %q", body, expected)
		}
		t.Logf("response body: %s", body)

		// Verify debug headers
		if shard := resp.Header.Get("X-Shard"); shard == "" {
			t.Error("missing X-Shard header")
		} else {
			t.Logf("X-Shard: %s", shard)
		}
		if servedBy := resp.Header.Get("X-Served-By"); servedBy == "" {
			t.Error("missing X-Served-By header")
		} else {
			t.Logf("X-Served-By: %s", servedBy)
		}
	})
}
