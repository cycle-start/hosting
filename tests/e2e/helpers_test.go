//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const coreAPIURL = "http://localhost:8090/api/v1"

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

// findNodeIPs queries the core API for node IPs matching a shard role.
func findNodeIPs(t *testing.T, clusterName, role string) []string {
	t.Helper()

	// Find cluster ID by listing regions/clusters
	resp, body, err := httpGetWithHost(coreAPIURL+"/regions", "")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("list regions: %v (status=%d body=%s)", err, resp.StatusCode, body)
	}

	var regions []struct {
		ID string `json:"id"`
	}
	json.Unmarshal([]byte(body), &regions)

	var clusterID string
	for _, r := range regions {
		cResp, cBody, err := httpGetWithHost(fmt.Sprintf("%s/regions/%s/clusters", coreAPIURL, r.ID), "")
		if err != nil || cResp.StatusCode != 200 {
			continue
		}
		var clusters []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		json.Unmarshal([]byte(cBody), &clusters)
		for _, c := range clusters {
			if c.Name == clusterName {
				clusterID = c.ID
				break
			}
		}
		if clusterID != "" {
			break
		}
	}
	if clusterID == "" {
		t.Fatalf("cluster %q not found", clusterName)
	}

	// List nodes in the cluster
	nResp, nBody, err := httpGetWithHost(fmt.Sprintf("%s/clusters/%s/nodes", coreAPIURL, clusterID), "")
	if err != nil || nResp.StatusCode != 200 {
		t.Fatalf("list nodes: %v", err)
	}

	var nodes []struct {
		IPAddress *string  `json:"ip_address"`
		Roles     []string `json:"roles"`
	}
	json.Unmarshal([]byte(nBody), &nodes)

	var ips []string
	for _, n := range nodes {
		if n.IPAddress == nil {
			continue
		}
		for _, r := range n.Roles {
			if r == role {
				ips = append(ips, *n.IPAddress)
				break
			}
		}
	}

	if len(ips) == 0 {
		t.Fatalf("no nodes with role %q found in cluster %q", role, clusterName)
	}
	return ips
}

// noRedirectClient is an HTTP client that does not follow redirects.
var noRedirectClient = &http.Client{
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
