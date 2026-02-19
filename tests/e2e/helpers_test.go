package e2e

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// coreAPIURL is the base URL for the hosting core API.
// Override with CORE_API_URL env var.
var coreAPIURL = "https://api.hosting.test/api/v1"

// webTrafficURL is the base URL for testing web traffic through HAProxy.
// Override with WEB_TRAFFIC_URL env var.
var webTrafficURL = "https://10.10.10.70"

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

// setAPIKey adds the Authorization: Bearer header to a request.
func setAPIKey(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+apiKey())
}

// sshExec runs a command on a VM via SSH and returns stdout.
func sshExec(t *testing.T, ip string, cmd string) string {
	t.Helper()

	keyPath := os.Getenv("SSH_KEY_PATH")
	if keyPath == "" {
		keyPath = os.ExpandEnv("${HOME}/.ssh/id_rsa")
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
	type nodeEntry struct {
		ip         string
		shardIndex int
	}
	var entries []nodeEntry
	for _, n := range nodes {
		if sid, _ := n["shard_id"].(string); sid == shardID {
			if ip, ok := n["ip_address"].(string); ok && ip != "" {
				// Strip CIDR suffix if present (e.g., "10.10.10.50/32" -> "10.10.10.50").
				if idx := strings.Index(ip, "/"); idx != -1 {
					ip = ip[:idx]
				}
				si := 0
				if v, ok := n["shard_index"].(float64); ok {
					si = int(v)
				}
				entries = append(entries, nodeEntry{ip: ip, shardIndex: si})
			}
		}
	}

	// Sort by shard_index so the primary (index 1) is first.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].shardIndex < entries[j].shardIndex
	})

	var ips []string
	for _, e := range entries {
		ips = append(ips, e.ip)
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

// httpPatch performs an HTTP PATCH with a JSON body.
func httpPatch(t *testing.T, url string, body interface{}) (*http.Response, string) {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal PATCH body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(http.MethodPatch, url, reqBody)
	if err != nil {
		t.Fatalf("create PATCH request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	setAPIKey(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PATCH %s: %v", url, err)
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

	// Check if brand already exists by listing brands and matching by name.
	resp, body := httpGet(t, coreAPIURL+"/brands")
	if resp.StatusCode == 200 {
		brands := parsePaginatedItems(t, body)
		for _, b := range brands {
			if name, _ := b["name"].(string); name == "E2E Test Brand" {
				return b["id"].(string)
			}
		}
	}

	// Create brand.
	resp, body = httpPost(t, coreAPIURL+"/brands", map[string]interface{}{
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

// findShardIDByRole returns the shard ID for a given role, or empty string if not found.
func findShardIDByRole(t *testing.T, clusterID, role string) string {
	t.Helper()
	resp, body := httpGet(t, fmt.Sprintf("%s/clusters/%s/shards", coreAPIURL, clusterID))
	if resp.StatusCode != 200 {
		return ""
	}
	shards := parsePaginatedItems(t, body)
	for _, s := range shards {
		if r, _ := s["role"].(string); r == role {
			id, _ := s["id"].(string)
			return id
		}
	}
	return ""
}

// findValkeyShardID returns the shard ID for the valkey role in the cluster.
func findValkeyShardID(t *testing.T, clusterID string) string {
	t.Helper()
	return findShardIDByRole(t, clusterID, "valkey")
}

// findEmailShardID returns the shard ID for the email role in the cluster.
func findEmailShardID(t *testing.T, clusterID string) string {
	t.Helper()
	return findShardIDByRole(t, clusterID, "email")
}

// findDNSShardID returns the shard ID for the dns role in the cluster.
func findDNSShardID(t *testing.T, clusterID string) string {
	t.Helper()
	return findShardIDByRole(t, clusterID, "dns")
}

// createTestWebroot creates a webroot on a tenant and waits for it to become active.
func createTestWebroot(t *testing.T, tenantID, name, runtime, version string) string {
	t.Helper()
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/webroots", coreAPIURL, tenantID), map[string]interface{}{
		"name":            name,
		"runtime":         runtime,
		"runtime_version": version,
	})
	require.Equal(t, 202, resp.StatusCode, "create webroot: %s", body)
	webroot := parseJSON(t, body)
	webrootID := webroot["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/webroots/"+webrootID) })
	waitForStatus(t, coreAPIURL+"/webroots/"+webrootID, "active", provisionTimeout)
	return webrootID
}

// createTestFQDN creates an FQDN on a webroot and waits for it to become active.
func createTestFQDN(t *testing.T, webrootID, fqdn string) string {
	t.Helper()
	resp, body := httpPost(t, fmt.Sprintf("%s/webroots/%s/fqdns", coreAPIURL, webrootID), map[string]interface{}{
		"fqdn": fqdn,
	})
	require.Equal(t, 202, resp.StatusCode, "create FQDN: %s", body)
	f := parseJSON(t, body)
	fqdnID := f["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/fqdns/"+fqdnID) })
	waitForStatus(t, coreAPIURL+"/fqdns/"+fqdnID, "active", provisionTimeout)
	return fqdnID
}

// createTestZone creates a zone and waits for it to become active.
func createTestZone(t *testing.T, tenantID, regionID, name string) string {
	t.Helper()
	resp, body := httpPost(t, coreAPIURL+"/zones", map[string]interface{}{
		"name":      name,
		"tenant_id": tenantID,
		"region_id": regionID,
	})
	require.Equal(t, 202, resp.StatusCode, "create zone: %s", body)
	zone := parseJSON(t, body)
	zoneID := zone["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/zones/"+zoneID) })
	waitForStatus(t, coreAPIURL+"/zones/"+zoneID, "active", provisionTimeout)
	return zoneID
}

// createTestDatabase creates a database and waits for it to become active.
func createTestDatabase(t *testing.T, tenantID, shardID, name string) (string, string) {
	t.Helper()
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/databases", coreAPIURL, tenantID), map[string]interface{}{
		"name":     name,
		"shard_id": shardID,
	})
	require.Equal(t, 202, resp.StatusCode, "create database: %s", body)
	db := parseJSON(t, body)
	dbID := db["id"].(string)
	dbName := db["name"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/databases/"+dbID) })
	waitForStatus(t, coreAPIURL+"/databases/"+dbID, "active", provisionTimeout)
	return dbID, dbName
}

// createTestValkeyInstance creates a Valkey instance and waits for it to become active.
func createTestValkeyInstance(t *testing.T, tenantID, shardID, name string) string {
	t.Helper()
	resp, body := httpPost(t, fmt.Sprintf("%s/tenants/%s/valkey-instances", coreAPIURL, tenantID), map[string]interface{}{
		"name":          name,
		"shard_id":      shardID,
		"max_memory_mb": 64,
	})
	require.Equal(t, 202, resp.StatusCode, "create valkey instance: %s", body)
	inst := parseJSON(t, body)
	instID := inst["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/valkey-instances/"+instID) })
	waitForStatus(t, coreAPIURL+"/valkey-instances/"+instID, "active", provisionTimeout)
	return instID
}

// httpDoWithKey performs an HTTP request using a specific API key.
func httpDoWithKey(t *testing.T, method, url string, body interface{}, key string) (*http.Response, string) {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		t.Fatalf("create %s request %s: %v", method, url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// httpGetWithKey performs an HTTP GET using a specific API key.
func httpGetWithKey(t *testing.T, url, key string) (*http.Response, string) {
	return httpDoWithKey(t, http.MethodGet, url, nil, key)
}

// httpPostWithKey performs an HTTP POST using a specific API key.
func httpPostWithKey(t *testing.T, url string, body interface{}, key string) (*http.Response, string) {
	return httpDoWithKey(t, http.MethodPost, url, body, key)
}

// httpDeleteWithKey performs an HTTP DELETE using a specific API key.
func httpDeleteWithKey(t *testing.T, url, key string) (*http.Response, string) {
	return httpDoWithKey(t, http.MethodDelete, url, nil, key)
}

// generateSSHKeyPair generates an RSA SSH key pair for testing.
// Returns the public key in authorized_keys format.
func generateSSHKeyPair(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	pub, err := ssh.NewPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("create SSH public key: %v", err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
}

// generateSelfSignedCert generates a self-signed TLS certificate for testing.
func generateSelfSignedCert(t *testing.T, cn string) (certPEM, keyPEM string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(24 * time.Hour),
		DNSNames:     []string{cn},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}

	var certBuf bytes.Buffer
	pem.Encode(&certBuf, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	var keyBuf bytes.Buffer
	pem.Encode(&keyBuf, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	return certBuf.String(), keyBuf.String()
}

// digQuery performs a DNS query using the dig command against a specific nameserver.
func digQuery(t *testing.T, nameserverIP, recordType, name string) string {
	t.Helper()
	out, err := exec.Command("dig", "@"+nameserverIP, recordType, name, "+short").CombinedOutput()
	if err != nil {
		t.Logf("dig query failed (may be expected): %v: %s", err, string(out))
		return ""
	}
	return strings.TrimSpace(string(out))
}

const migrationTimeout = 10 * time.Minute

// httpPostWithHost performs an HTTP POST with a JSON body and custom Host header.
// It routes through the LB (webTrafficURL), not the API.
func httpPostWithHost(t *testing.T, url, host string, body interface{}) (*http.Response, string) {
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
	if host != "" {
		req.Host = host
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		t.Fatalf("POST %s (Host: %s): %v", url, host, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, string(b)
}

// httpGetWithHostErr performs an HTTP GET with a custom Host header.
// Returns error instead of calling t.Fatal.
func httpGetWithHostErr(url, host string) (*http.Response, string, error) {
	return httpGetWithHost(url, host)
}

// createDatabaseUser creates a database user and waits for it to become active.
// Returns the user ID.
func createDatabaseUser(t *testing.T, dbID, username, password string) string {
	t.Helper()
	resp, body := httpPost(t, fmt.Sprintf("%s/databases/%s/users", coreAPIURL, dbID), map[string]interface{}{
		"username":   username,
		"password":   password,
		"privileges": []string{"ALL"},
	})
	require.Equal(t, 202, resp.StatusCode, "create database user: %s", body)
	user := parseJSON(t, body)
	userID := user["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/database-users/"+userID) })
	waitForStatus(t, coreAPIURL+"/database-users/"+userID, "active", provisionTimeout)
	return userID
}

// createTestDaemon creates a daemon on a webroot and registers cleanup.
// Returns the daemon ID and parsed response body.
func createTestDaemon(t *testing.T, webrootID string, body map[string]interface{}) (string, map[string]interface{}) {
	t.Helper()
	resp, respBody := httpPost(t, fmt.Sprintf("%s/webroots/%s/daemons", coreAPIURL, webrootID), body)
	require.Equal(t, 202, resp.StatusCode, "create daemon: %s", respBody)
	daemon := parseJSON(t, respBody)
	daemonID := daemon["id"].(string)
	t.Cleanup(func() { httpDelete(t, coreAPIURL+"/daemons/"+daemonID) })
	return daemonID, daemon
}

// findFixtureTarball locates the Laravel Reverb fixture tarball.
// Skips the test if the tarball is not found.
func findFixtureTarball(t *testing.T) string {
	t.Helper()
	path := "../../.build/laravel-reverb.tar.gz"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Laravel fixture tarball not found — run 'just build-laravel-fixture' first")
	}
	return path
}

// scpFile copies a local file to a remote VM via scp.
func scpFile(t *testing.T, ip, localPath, remotePath string) {
	t.Helper()
	keyPath := os.Getenv("SSH_KEY_PATH")
	if keyPath == "" {
		keyPath = os.ExpandEnv("${HOME}/.ssh/id_rsa")
	}
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-i", keyPath,
		localPath,
		"ubuntu@" + ip + ":" + remotePath,
	}
	out, err := exec.Command("scp", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("scp to %s:%s: %v\n%s", ip, remotePath, err, string(out))
	}
}

// uploadFixture copies and extracts the Laravel fixture tarball to the webroot directory.
func uploadFixture(t *testing.T, nodeIP, webrootPath, tarballPath, tenantName string) {
	t.Helper()
	// Copy tarball to /tmp on the node.
	scpFile(t, nodeIP, tarballPath, "/tmp/laravel-reverb.tar.gz")
	// Extract into the webroot directory.
	sshExec(t, nodeIP, fmt.Sprintf(
		"sudo tar -xzf /tmp/laravel-reverb.tar.gz -C %s && sudo chown -R %s:%s %s && sudo chmod -R 775 %s/storage %s/bootstrap/cache",
		webrootPath, tenantName, tenantName, webrootPath,
		webrootPath, webrootPath,
	))
	sshExec(t, nodeIP, "rm -f /tmp/laravel-reverb.tar.gz")
}

// generateLaravelEnv writes a .env file to the webroot directory on the node.
func generateLaravelEnv(t *testing.T, nodeIP, webrootPath, tenantName string, envVars map[string]string) {
	t.Helper()
	var lines []string
	// Sort keys for deterministic output.
	keys := make([]string, 0, len(envVars))
	for k := range envVars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%s=%s", k, envVars[k]))
	}
	content := strings.Join(lines, "\n") + "\n"

	// Write via ssh, escaping the content.
	sshExec(t, nodeIP, fmt.Sprintf(
		"cat <<'ENVEOF' | sudo tee %s/.env > /dev/null\n%sENVEOF",
		webrootPath, content,
	))
	sshExec(t, nodeIP, fmt.Sprintf("sudo chown %s:%s %s/.env", tenantName, tenantName, webrootPath))
}
