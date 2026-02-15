package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/edvin/hosting/internal/core"
	"github.com/stretchr/testify/require"
)

// TestLaravelReverbQueue exercises the full stack: PHP-FPM serves HTTP,
// a queue worker daemon processes jobs, and a Reverb WebSocket daemon
// broadcasts events to connected clients.
func TestLaravelReverbQueue(t *testing.T) {
	tarball := findFixtureTarball(t)

	// ---------------------------------------------------------------
	// Phase 1: Provision infrastructure
	// ---------------------------------------------------------------
	tenantID, _, clusterID, _, dbShardID := createTestTenant(t, "e2e-reverb")
	if dbShardID == "" {
		t.Skip("no database shard found in cluster; skipping")
	}

	// Get tenant details (name, uid).
	resp, body := httpGet(t, coreAPIURL+"/tenants/"+tenantID)
	require.Equal(t, 200, resp.StatusCode, "get tenant: %s", body)
	tenant := parseJSON(t, body)
	tenantName := tenant["name"].(string)
	tenantUID := int(tenant["uid"].(float64))
	t.Logf("tenant: name=%s uid=%d", tenantName, tenantUID)

	// Create database + user.
	dbName := "e2e_reverb_db"
	dbID := createTestDatabase(t, tenantID, dbShardID, dbName)
	dbUserID := createDatabaseUser(t, dbID, "e2e_reverb", "ReverbT3st!Pass")
	_ = dbUserID
	t.Logf("database %s active, user created", dbID)

	// Find DB node IP for MySQL host.
	dbNodeIPs := findNodeIPsByRole(t, clusterID, "database")
	dbHost := dbNodeIPs[0]
	t.Logf("database host: %s", dbHost)

	// Create webroot with public_folder.
	webrootName := "reverb-app"
	resp, body = httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":            webrootName,
		"runtime":         "php",
		"runtime_version": "8.5",
		"public_folder":   "public",
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/webroots/"+webrootID) })
	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	t.Logf("webroot %s active", webrootID)

	// Create FQDN.
	fqdn := "reverb-e2e.hosting.test."
	fqdnID := createTestFQDN(t, webrootID, fqdn)
	t.Logf("fqdn %s active (id=%s)", fqdn, fqdnID)
	fqdnHost := strings.TrimSuffix(fqdn, ".") // "reverb-e2e.hosting.test"

	// ---------------------------------------------------------------
	// Phase 2: Upload Laravel project
	// ---------------------------------------------------------------
	webNodeIPs := findNodeIPsByRole(t, clusterID, "web")
	webNodeIP := webNodeIPs[0]
	webrootPath := fmt.Sprintf("/var/www/storage/%s/webroots/%s", tenantName, webrootName)
	uploadFixture(t, webNodeIP, webrootPath, tarball, tenantName)
	t.Logf("fixture uploaded to %s:%s", webNodeIP, webrootPath)

	// ---------------------------------------------------------------
	// Phase 3: Create Reverb daemon (get proxy_port)
	// ---------------------------------------------------------------
	proxyPath := "/app"
	reverbDaemonID, reverbDaemon := createTestDaemon(t, webrootID, map[string]interface{}{
		"command":    `bash -c 'exec php artisan reverb:start --host=:: --port=$PORT'`,
		"proxy_path": proxyPath,
	})
	reverbPort := int(reverbDaemon["proxy_port"].(float64))
	t.Logf("reverb daemon %s created, proxy_port=%d", reverbDaemonID, reverbPort)

	// ---------------------------------------------------------------
	// Phase 4: Configure .env and ULA
	// ---------------------------------------------------------------

	// Get daemon's assigned node_id.
	// The daemon may not have node_id yet (set during workflow). Poll for it.
	var daemonNodeID string
	deadline := time.Now().Add(provisionTimeout)
	for time.Now().Before(deadline) {
		resp, body = httpGet(t, coreAPIURL+"/daemons/"+reverbDaemonID)
		require.Equal(t, 200, resp.StatusCode, "get daemon: %s", body)
		d := parseJSON(t, body)
		if nid, ok := d["node_id"].(string); ok && nid != "" {
			daemonNodeID = nid
			break
		}
		time.Sleep(2 * time.Second)
	}
	require.NotEmpty(t, daemonNodeID, "daemon never got a node_id")

	// Get node details to find shard_index and cluster_id.
	resp, body = httpGet(t, fmt.Sprintf("%s/clusters/%s/nodes", coreAPIURL, clusterID))
	require.Equal(t, 200, resp.StatusCode, body)
	nodes := parsePaginatedItems(t, body)
	var daemonNodeShardIndex int
	var daemonNodeIP string
	for _, n := range nodes {
		if nid, _ := n["id"].(string); nid == daemonNodeID {
			if si, ok := n["shard_index"].(float64); ok {
				daemonNodeShardIndex = int(si)
			}
			if ip, ok := n["ip_address"].(string); ok && ip != "" {
				daemonNodeIP = ip
				if idx := strings.Index(daemonNodeIP, "/"); idx != -1 {
					daemonNodeIP = daemonNodeIP[:idx]
				}
			}
			break
		}
	}
	require.NotEmpty(t, daemonNodeIP, "could not find daemon node IP")
	t.Logf("daemon node: id=%s ip=%s shard_index=%d", daemonNodeID, daemonNodeIP, daemonNodeShardIndex)

	// Compute ULA and add to loopback on the daemon's node.
	ula := core.ComputeTenantULA(clusterID, daemonNodeShardIndex, tenantUID)
	t.Logf("computed ULA: %s", ula)
	sshExec(t, daemonNodeIP, fmt.Sprintf("sudo ip -6 addr add %s/128 dev lo || true", ula))
	t.Cleanup(func() {
		sshExec(t, daemonNodeIP, fmt.Sprintf("sudo ip -6 addr del %s/128 dev lo || true", ula))
	})

	// Write .env.
	generateLaravelEnv(t, daemonNodeIP, webrootPath, tenantName, map[string]string{
		"APP_NAME":             "LaravelReverbE2E",
		"APP_ENV":              "testing",
		"APP_DEBUG":            "true",
		"APP_URL":              fmt.Sprintf("https://%s", fqdnHost),
		"DB_CONNECTION":        "mysql",
		"DB_HOST":              dbHost,
		"DB_PORT":              "3306",
		"DB_DATABASE":          dbName,
		"DB_USERNAME":          "e2e_reverb",
		"DB_PASSWORD":          "ReverbT3st!Pass",
		"BROADCAST_CONNECTION": "reverb",
		"QUEUE_CONNECTION":     "database",
		"REVERB_APP_ID":        "e2e-test",
		"REVERB_APP_KEY":       "e2e-test-key",
		"REVERB_APP_SECRET":    "e2e-test-secret",
		"REVERB_HOST":          "127.0.0.1",
		"REVERB_PORT":          fmt.Sprintf("%d", reverbPort),
		"REVERB_SCHEME":        "http",
	})
	t.Logf(".env written")

	// Generate app key.
	sshExec(t, daemonNodeIP, fmt.Sprintf(
		"sudo -u %s php %s/artisan key:generate --force",
		tenantName, webrootPath,
	))
	t.Logf("app key generated")

	// If fixture was uploaded to only one node but there are multiple web nodes,
	// CephFS shared storage means it's visible on all nodes.

	// ---------------------------------------------------------------
	// Phase 5: Create queue worker daemon, wait for both daemons
	// ---------------------------------------------------------------
	queueDaemonID, _ := createTestDaemon(t, webrootID, map[string]interface{}{
		"command": "php artisan queue:work --sleep=1 --tries=3 --timeout=30",
	})
	t.Logf("queue worker daemon %s created", queueDaemonID)

	// Wait for both daemons to become active.
	waitForStatus(t, coreAPIURL+"/daemons/"+reverbDaemonID, "active", provisionTimeout)
	t.Logf("reverb daemon active")
	waitForStatus(t, coreAPIURL+"/daemons/"+queueDaemonID, "active", provisionTimeout)
	t.Logf("queue worker daemon active")

	// Give daemons a moment to stabilize (supervisord starts the process).
	time.Sleep(5 * time.Second)

	// ---------------------------------------------------------------
	// Phase 6: Run migrations
	// ---------------------------------------------------------------
	setupURL := fmt.Sprintf("%s/api/setup", webTrafficURL)
	var lastSetupErr string
	setupDeadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(setupDeadline) {
		resp, body = httpPostWithHost(t, setupURL, fqdnHost, nil)
		if resp.StatusCode == 200 {
			t.Logf("migrations complete: %s", body)
			break
		}
		lastSetupErr = fmt.Sprintf("status=%d body=%s", resp.StatusCode, body)
		time.Sleep(3 * time.Second)
	}
	require.Equal(t, 200, resp.StatusCode, "setup failed: %s", lastSetupErr)

	// ---------------------------------------------------------------
	// Phase 7: Verification subtests
	// ---------------------------------------------------------------
	reverbKey := "e2e-test-key"

	t.Run("websocket_handshake", func(t *testing.T) {
		wsURL := fmt.Sprintf("wss://10.10.10.2/app/%s?protocol=7", reverbKey)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			Host: fqdnHost,
		})
		require.NoError(t, err, "websocket dial")
		defer conn.CloseNow()

		// Read the connection_established message.
		_, msg, err := conn.Read(ctx)
		require.NoError(t, err, "read ws message")

		var pusherMsg map[string]interface{}
		require.NoError(t, json.Unmarshal(msg, &pusherMsg), "parse pusher message")
		require.Equal(t, "pusher:connection_established", pusherMsg["event"],
			"expected connection_established, got: %s", string(msg))
		t.Logf("websocket handshake OK: %s", string(msg))

		conn.Close(websocket.StatusNormalClosure, "done")
	})

	t.Run("queue_processing", func(t *testing.T) {
		marker := fmt.Sprintf("queue-test-%d", time.Now().UnixNano())

		// Dispatch a job via HTTP.
		dispatchURL := fmt.Sprintf("%s/api/dispatch-test", webTrafficURL)
		resp, body := httpPostWithHost(t, dispatchURL, fqdnHost, map[string]interface{}{
			"marker": marker,
		})
		require.Equal(t, 200, resp.StatusCode, "dispatch: %s", body)
		t.Logf("job dispatched with marker: %s", marker)

		// Poll check-result until the marker matches.
		checkURL := fmt.Sprintf("%s/api/check-result", webTrafficURL)
		pollDeadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(pollDeadline) {
			resp, body, err := httpGetWithHost(checkURL, fqdnHost)
			if err == nil && resp.StatusCode == 200 {
				result := parseJSON(t, body)
				if m, _ := result["marker"].(string); m == marker {
					t.Logf("queue processing verified: marker=%s", marker)
					return
				}
			}
			time.Sleep(1 * time.Second)
		}
		t.Fatalf("timed out waiting for queue job to process marker %s", marker)
	})

	t.Run("full_broadcast_flow", func(t *testing.T) {
		marker := fmt.Sprintf("broadcast-test-%d", time.Now().UnixNano())

		// Connect WebSocket.
		wsURL := fmt.Sprintf("wss://10.10.10.2/app/%s?protocol=7", reverbKey)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
			Host: fqdnHost,
		})
		require.NoError(t, err, "websocket dial")
		defer conn.CloseNow()

		// Read connection_established.
		_, msg, err := conn.Read(ctx)
		require.NoError(t, err, "read connection_established")
		t.Logf("connected: %s", string(msg))

		// Subscribe to test-channel.
		subscribeMsg, _ := json.Marshal(map[string]interface{}{
			"event": "pusher:subscribe",
			"data": map[string]interface{}{
				"channel": "test-channel",
			},
		})
		require.NoError(t, conn.Write(ctx, websocket.MessageText, subscribeMsg), "send subscribe")

		// Read subscription_succeeded.
		_, msg, err = conn.Read(ctx)
		require.NoError(t, err, "read subscription response")
		t.Logf("subscription response: %s", string(msg))

		// Dispatch a job that broadcasts.
		dispatchURL := fmt.Sprintf("%s/api/dispatch-test", webTrafficURL)
		resp, body := httpPostWithHost(t, dispatchURL, fqdnHost, map[string]interface{}{
			"marker": marker,
		})
		require.Equal(t, 200, resp.StatusCode, "dispatch: %s", body)
		t.Logf("job dispatched with marker: %s", marker)

		// Wait for the broadcast event on the WebSocket.
		broadcastDeadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(broadcastDeadline) {
			_, msg, err := conn.Read(ctx)
			if err != nil {
				t.Fatalf("websocket read error: %v", err)
			}

			var event map[string]interface{}
			if err := json.Unmarshal(msg, &event); err != nil {
				t.Logf("non-JSON ws message: %s", string(msg))
				continue
			}

			eventName, _ := event["event"].(string)
			t.Logf("ws event: %s", eventName)

			if eventName == "test.broadcast" {
				// Parse the data field (it's a JSON string inside the event).
				dataStr, _ := event["data"].(string)
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(dataStr), &data); err != nil {
					t.Fatalf("parse broadcast data: %v (raw: %s)", err, dataStr)
				}
				if m, _ := data["marker"].(string); m == marker {
					t.Logf("broadcast received with correct marker: %s", marker)
					conn.Close(websocket.StatusNormalClosure, "done")
					return
				}
			}
		}
		t.Fatalf("timed out waiting for broadcast event with marker %s", marker)
	})
}
