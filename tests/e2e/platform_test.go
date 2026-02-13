//go:build e2e

package e2e

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/edvin/hosting/internal/hostctl"
)

const (
	seedConfigPath   = "../../seeds/dev-tenants.yaml"
	provisionTimeout = 5 * time.Minute
	seedTimeout      = 5 * time.Minute
	httpTimeout      = 60 * time.Second
	clusterName      = "vm-cluster-1"
)

func clusterConfigPath() string {
	if p := os.Getenv("CLUSTER_CONFIG"); p != "" {
		return p
	}
	return "../../terraform/cluster.yaml"
}

func TestPlatformE2E(t *testing.T) {
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
		if err := hostctl.ClusterApply(clusterConfigPath(), provisionTimeout); err != nil {
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
		ips := findNodeIPs(t, clusterName, "web")
		if len(ips) < 2 {
			t.Skip("need at least 2 web nodes to test shared storage")
		}
		nodeA, nodeB := ips[0], ips[1]
		t.Logf("node A: %s, node B: %s", nodeA, nodeB)

		// Write a unique file on node A.
		testFile := "/var/www/storage/.e2e-shared-storage-test"
		marker := "shared-storage-ok"
		sshExec(t, nodeA, "echo '"+marker+"' | sudo tee "+testFile)

		// Read it from node B.
		got := sshExec(t, nodeB, "cat "+testFile)
		if !strings.Contains(got, marker) {
			t.Fatalf("file written on node A not visible on node B: got %q, want %q", got, marker)
		}
		t.Logf("cross-node read OK: %q", strings.TrimSpace(got))

		// Clean up.
		sshExec(t, nodeA, "sudo rm -f "+testFile)
	})

	t.Run("web_traffic", func(t *testing.T) {
		ips := findNodeIPs(t, clusterName, "web")
		nodeIP := ips[0]
		t.Logf("writing index.php to web node %s", nodeIP)

		phpContent := `<?php echo "Hello from hosting platform"; ?>`
		sshExec(t, nodeIP, "sudo mkdir -p /var/www/storage/acme-corp/main-site/public")
		sshExec(t, nodeIP, "echo '"+phpContent+"' | sudo tee /var/www/storage/acme-corp/main-site/public/index.php")

		// Wait for the page to be served through HAProxy
		resp, body := waitForHTTP(t, "http://localhost:80", "acme.hosting.test", httpTimeout)

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
