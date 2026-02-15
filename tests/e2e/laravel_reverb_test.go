package e2e

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
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
	t.Logf("tenant: name=%s", tenantName)

	// Create database + user.
	dbID, dbName := createTestDatabase(t, tenantID, dbShardID, "e2e_reverb_db")
	dbUsername := dbName + "_app"
	dbPassword := "ReverbT3stPass99"
	createDatabaseUser(t, dbID, dbUsername, dbPassword)
	t.Logf("database %s (name=%s) active, user created", dbID, dbName)

	// Find DB node IP for MySQL host.
	dbNodeIPs := findNodeIPsByRole(t, clusterID, "database")
	dbHost := dbNodeIPs[0]
	t.Logf("database host: %s", dbHost)

	// Create webroot with public_folder.
	resp, body = httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":            "reverb-app",
		"runtime":         "php",
		"runtime_version": "8.5",
		"public_folder":   "public",
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	webrootName := webroot["name"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/webroots/"+webrootID) })
	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	t.Logf("webroot %s active", webrootID)

	// Create FQDN (unique per run to avoid stale soft-deleted records).
	fqdn := fmt.Sprintf("reverb-%d.hosting.test.", time.Now().UnixNano())
	fqdnID := createTestFQDN(t, webrootID, fqdn)
	t.Logf("fqdn %s active (id=%s)", fqdn, fqdnID)
	fqdnHost := strings.TrimSuffix(fqdn, ".")

	// ---------------------------------------------------------------
	// Phase 2: Upload Laravel project + .env + run migrations
	// ---------------------------------------------------------------
	webNodeIPs := findNodeIPsByRole(t, clusterID, "web")
	webrootPath := fmt.Sprintf("/var/www/storage/%s/webroots/%s", tenantName, webrootName)

	// Upload to all web nodes (CephFS not available in dev environment).
	for _, ip := range webNodeIPs {
		uploadFixture(t, ip, webrootPath, tarball, tenantName)
		t.Logf("fixture uploaded to %s:%s", ip, webrootPath)
	}

	// Generate a random Laravel APP_KEY (32 bytes, base64-encoded).
	keyBytes := make([]byte, 32)
	_, err := rand.Read(keyBytes)
	require.NoError(t, err, "generate app key")
	appKey := "base64:" + base64.StdEncoding.EncodeToString(keyBytes)

	// Add /etc/hosts entry on each web node so the PHP broadcasting client
	// can reach the Reverb server through nginx (localhost). The Pusher SDK
	// connects to REVERB_HOST:REVERB_PORT — by resolving the FQDN to 127.0.0.1,
	// the request goes through nginx's `location /app` proxy to Reverb.
	for _, ip := range webNodeIPs {
		sshExec(t, ip, fmt.Sprintf("echo '127.0.0.1 %s' | sudo tee -a /etc/hosts > /dev/null", fqdnHost))
	}
	t.Cleanup(func() {
		for _, ip := range webNodeIPs {
			sshExec(t, ip, fmt.Sprintf("sudo sed -i '/%s/d' /etc/hosts", fqdnHost))
		}
	})

	// Write .env to ALL web nodes before creating daemons.
	// REVERB_HOST/PORT/SCHEME route the PHP broadcasting client through nginx
	// on localhost, which proxies to the Reverb daemon via ULA.
	envVars := map[string]string{
		"APP_NAME":             "LaravelReverbE2E",
		"APP_ENV":              "testing",
		"APP_DEBUG":            "true",
		"APP_KEY":              appKey,
		"APP_URL":              fmt.Sprintf("https://%s", fqdnHost),
		"DB_CONNECTION":        "mysql",
		"DB_HOST":              dbHost,
		"DB_PORT":              "3306",
		"DB_DATABASE":          dbName,
		"DB_USERNAME":          dbUsername,
		"DB_PASSWORD":          dbPassword,
		"BROADCAST_CONNECTION": "reverb",
		"QUEUE_CONNECTION":     "database",
		"CACHE_STORE":          "file",
		"REVERB_APP_ID":        "e2e-test",
		"REVERB_APP_KEY":       "e2e-test-key",
		"REVERB_APP_SECRET":    "e2e-test-secret",
		"REVERB_HOST":          fqdnHost,
		"REVERB_PORT":          "80",
		"REVERB_SCHEME":        "http",
	}
	for _, ip := range webNodeIPs {
		generateLaravelEnv(t, ip, webrootPath, tenantName, envVars)
	}
	t.Logf(".env written to %d web nodes", len(webNodeIPs))

	// Run migrations BEFORE creating daemons (queue worker needs the jobs table).
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
	// Phase 3: Create daemons
	// ---------------------------------------------------------------
	// Reverb WebSocket daemon — uses [$HOST] for proper IPv6 URI formatting.
	// ReactPHP requires IPv6 addresses in bracket notation within URIs.
	proxyPath := "/app"
	reverbDaemonID, _ := createTestDaemon(t, webrootID, map[string]interface{}{
		"command":    `bash -c 'exec php artisan reverb:start --host="[$HOST]" --port="$PORT"'`,
		"proxy_path": proxyPath,
	})
	t.Logf("reverb daemon %s created", reverbDaemonID)

	// Queue worker daemon (no proxy_path — pure background worker).
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
	// Phase 4: Verification subtests
	// ---------------------------------------------------------------
	reverbKey := "e2e-test-key"

	t.Run("websocket_handshake", func(t *testing.T) {
		wsURL := fmt.Sprintf("wss://10.10.10.70/app/%s?protocol=7", reverbKey)
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
		wsURL := fmt.Sprintf("wss://10.10.10.70/app/%s?protocol=7", reverbKey)
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
