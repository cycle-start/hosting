package e2e

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// coreAPIURL is the base URL for the hosting core API.
// Override with CORE_API_URL env var.
var coreAPIURL = "https://api.hosting.test/api/v1"

// webTrafficURL is the base URL for testing web traffic through HAProxy.
// Override with WEB_TRAFFIC_URL env var.
var webTrafficURL = "https://hosting.test"

// tlsTransport skips certificate verification for self-signed dev certs.
var tlsTransport = &http.Transport{
	TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
}

func TestMain(m *testing.M) {
	if os.Getenv("HOSTING_E2E") == "" {
		fmt.Println("Skipping e2e tests (set HOSTING_E2E=1 to run)")
		os.Exit(0)
	}
	if u := os.Getenv("CORE_API_URL"); u != "" {
		coreAPIURL = u
	}
	if u := os.Getenv("WEB_TRAFFIC_URL"); u != "" {
		webTrafficURL = u
	}
	http.DefaultTransport = tlsTransport
	os.Exit(m.Run())
}

// apiKey returns the API key for authenticating with the core API.
// Set via HOSTING_API_KEY env var; defaults to the dev test key.
func apiKey() string {
	if k := os.Getenv("HOSTING_API_KEY"); k != "" {
		return k
	}
	return "hst_dev_e2e_test_key_00000000"
}

// setAPIKey adds the X-API-Key header to a request.
func setAPIKey(req *http.Request) {
	req.Header.Set("X-API-Key", apiKey())
}

// sshExec runs a command on a VM via SSH and returns stdout.
func sshExec(t *testing.T, ip string, cmd string) string {
	t.Helper()

	keyPath := os.Getenv("SSH_KEY_PATH")
	if keyPath == "" {
		keyPath = os.ExpandEnv("${HOME}/.ssh/hosting-dev")
	}

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-i", keyPath,
		"ubuntu@" + ip,
		cmd,
	}
	out, err := exec.Command("ssh", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("ssh %s %q: %v\n%s", ip, cmd, err, string(out))
	}
	return string(out)
}

// findNodeIPsByRole returns all node IPs for the given role in a cluster.
// It finds the shard with the matching role, then filters nodes by shard_id.
func findNodeIPsByRole(t *testing.T, clusterID, role string) []string {
	t.Helper()

	// Find the shard ID for this role.
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	require.Equal(t, 200, resp.StatusCode, "list shards: %s", body)

	shards := parsePaginatedItems(t, body)
	var shardID string
	for _, s := range shards {
		if r, _ := s["role"].(string); r == role {
			shardID, _ = s["id"].(string)
			break
		}
	}
	if shardID == "" {
		t.Fatalf("no shard with role %q in cluster %s", role, clusterID)
	}

	// List all nodes in the cluster, filter by shard_id.
	resp, body = httpGet(t, fmt.Sprintf("%s/clusters/%s/nodes", coreAPIURL, clusterID))
	if resp.StatusCode != 200 {
		t.Fatalf("list nodes: status %d body=%s", resp.StatusCode, body)
	}

	nodes := parsePaginatedItems(t, body)
	var ips []string
	for _, n := range nodes {
		if sid, _ := n["shard_id"].(string); sid == shardID {
			if ip, ok := n["ip_address"].(string); ok && ip != "" {
				// Strip CIDR suffix if present (e.g., "10.10.10.50/32" -> "10.10.10.50").
				if idx := strings.Index(ip, "/"); idx != -1 {
					ip = ip[:idx]
				}
				ips = append(ips, ip)
			}
		}
	}

	if len(ips) == 0 {
		t.Fatalf("no nodes with role %q found in cluster %s", role, clusterID)
	}
	return ips
}

// noRedirectClient is an HTTP client that does not follow redirects.
var noRedirectClient = &http.Client{
	Transport: tlsTransport,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// httpGetWithHost performs an HTTP GET with a custom Host header.
// It does not follow redirects to avoid DNS resolution of virtual hostnames.
func httpGetWithHost(url, host string) (*http.Response, string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create request: %w", err)
	}
	setAPIKey(req)
	if host != "" {
		req.Host = host
	}

	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, "", fmt.Errorf("read body: %w", err)
	}

	return resp, string(body), nil
}

// waitForHTTP retries an HTTP GET with the given Host header until it
// succeeds (2xx) or the timeout elapses.
func waitForHTTP(t *testing.T, url, host string, timeout time.Duration) (*http.Response, string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		resp, body, err := httpGetWithHost(url, host)
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, body
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(body))
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("timed out waiting for %s (Host: %s): %v", url, host, lastErr)
	return nil, ""
}

// httpPost performs an HTTP POST with a JSON body, returns the response and body string.
func httpPost(t *testing.T, url string, body interface{}) (*http.Response, string) {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal POST body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(http.MethodPost, url, reqBody)
	if err != nil {
		t.Fatalf("create POST request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKey(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// httpPut performs an HTTP PUT with a JSON body.
func httpPut(t *testing.T, url string, body interface{}) (*http.Response, string) {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal PUT body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(http.MethodPut, url, reqBody)
	if err != nil {
		t.Fatalf("create PUT request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKey(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// httpDelete performs an HTTP DELETE.
func httpDelete(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("create DELETE request %s: %v", url, err)
	}
	setAPIKey(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// httpGet performs an HTTP GET and returns the response and body string.
func httpGet(t *testing.T, url string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("create GET request %s: %v", url, err)
	}
	setAPIKey(req)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// parseJSON unmarshals a JSON response body into a map.
func parseJSON(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("parse JSON: %v\nbody: %s", err, body)
	}
	return result
}

// parseJSONArray unmarshals a JSON array response body.
func parseJSONArray(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	var result []map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		t.Fatalf("parse JSON array: %v\nbody: %s", err, body)
	}
	return result
}

// parsePaginatedItems extracts the "items" array from a paginated response.
func parsePaginatedItems(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	wrapper := parseJSON(t, body)
	items, ok := wrapper["items"]
	if !ok {
		t.Fatalf("paginated response missing 'items' key: %s", body)
	}
	// Re-marshal and unmarshal the items to get []map[string]interface{}
	raw, _ := json.Marshal(items)
	var result []map[string]interface{}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("parse paginated items: %v", err)
	}
	return result
}

// waitForStatus polls a resource URL until its "status" field matches the
// desired value or the timeout elapses. Returns the final resource as a map.
func waitForStatus(t *testing.T, url, wantStatus string, timeout time.Duration) map[string]interface{} {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastStatus string
	var lastBody string

	for time.Now().Before(deadline) {
		resp, body := httpGet(t, url)
		if resp.StatusCode == http.StatusNotFound && wantStatus == "deleted" {
			// 404 means the resource has been fully removed; treat as "deleted".
			return map[string]interface{}{"status": "deleted"}
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			resource := parseJSON(t, body)
			status, _ := resource["status"].(string)
			lastStatus = status
			lastBody = body
			if status == wantStatus {
				return resource
			}
			if status == "failed" && wantStatus != "failed" {
				t.Fatalf("resource entered failed state while waiting for %q: %s", wantStatus, body)
			}
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatalf("timed out waiting for status %q at %s (last status=%q, body=%s)", wantStatus, url, lastStatus, lastBody)
	return nil
}

// findFirstRegionID returns the ID of the first region from the API.
func findFirstRegionID(t *testing.T) string {
	t.Helper()
	resp, body := httpGet(t, coreAPIURL+"/regions")
	if resp.StatusCode != 200 {
		t.Fatalf("list regions: status %d body=%s", resp.StatusCode, body)
	}
	regions := parsePaginatedItems(t, body)
	if len(regions) == 0 {
		t.Fatal("no regions found")
	}
	id, ok := regions[0]["id"].(string)
	if !ok || id == "" {
		t.Fatal("first region has no id")
	}
	return id
}

// findFirstCluster returns the ID of the first cluster in the given region.
func findFirstCluster(t *testing.T, regionID string) map[string]interface{} {
	t.Helper()
	resp, body := httpGet(t, fmt.Sprintf("%s/regions/%s/clusters", coreAPIURL, regionID))
	if resp.StatusCode != 200 {
		t.Fatalf("list clusters: status %d body=%s", resp.StatusCode, body)
	}
	clusters := parsePaginatedItems(t, body)
	if len(clusters) == 0 {
		t.Fatal("no clusters found")
	}
	return clusters[0]
}

// findShardByRole returns the first shard with the given role in a cluster.
func findShardByRole(t *testing.T, clusterID, role string) map[string]interface{} {
	t.Helper()
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	if resp.StatusCode != 200 {
		t.Fatalf("list shards: status %d body=%s", resp.StatusCode, body)
	}
	shards := parsePaginatedItems(t, body)
	for _, s := range shards {
		if r, _ := s["role"].(string); r == role {
			return s
		}
	}
	t.Fatalf("no shard with role %q in cluster %s", role, clusterID)
	return nil
}

// findOrCreateBrand ensures a test brand exists and returns its ID.
func findOrCreateBrand(t *testing.T) string {
	t.Helper()

	// Check if brand already exists.
	resp, body := httpGet(t, coreAPIURL+"/brands/e2e-brand")
	if resp.StatusCode == 200 {
		brand := parseJSON(t, body)
		return brand["id"].(string)
	}

	// Create brand.
	resp, body = httpPost(t, coreAPIURL+"/brands", map[string]interface{}{
		"id":               "e2e-brand",
		"name":             "E2E Test Brand",
		"base_hostname":    "e2e.hosting.test",
		"primary_ns":       "ns1.e2e.hosting.test",
		"secondary_ns":     "ns2.e2e.hosting.test",
		"hostmaster_email": "hostmaster@e2e.hosting.test",
	})
	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		t.Fatalf("create brand: status %d body=%s", resp.StatusCode, body)
	}
	brand := parseJSON(t, body)
	return brand["id"].(string)
}

// createTestTenant creates a tenant assigned to the first web shard and
// waits for it to become active. It registers a cleanup function that
// deletes the tenant when the test completes.
func createTestTenant(t *testing.T, name string) (tenantID, regionID, clusterID, webShardID, dbShardID string) {
	t.Helper()

	brandID := findOrCreateBrand(t)
	regionID = findFirstRegionID(t)
	cluster := findFirstCluster(t, regionID)
	clusterID, _ = cluster["id"].(string)

	webShard := findShardByRole(t, clusterID, "web")
	webShardID, _ = webShard["id"].(string)

	// Try to find a database shard; not all clusters have one.
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	if resp.StatusCode == 200 {
		shards := parsePaginatedItems(t, body)
		for _, s := range shards {
			if r, _ := s["role"].(string); r == "database" {
				dbShardID, _ = s["id"].(string)
				break
			}
		}
	}

	createResp, createBody := httpPost(t, coreAPIURL+"/tenants", map[string]interface{}{
		"brand_id":   brandID,
		"region_id":  regionID,
		"cluster_id": clusterID,
		"shard_id":   webShardID,
	})
	if createResp.StatusCode != 202 {
		t.Fatalf("create tenant %q: status %d body=%s", name, createResp.StatusCode, createBody)
	}
	tenant := parseJSON(t, createBody)
	tenantID, _ = tenant["id"].(string)

	t.Cleanup(func() {
		// Best-effort delete; ignore errors.
		httpDelete(t, coreAPIURL+"/tenants/"+tenantID)
	})

	waitForStatus(t, coreAPIURL+"/tenants/"+tenantID, "active", provisionTimeout)
	t.Logf("tenant %q active: %s", name, tenantID)

	return tenantID, regionID, clusterID, webShardID, dbShardID
}
