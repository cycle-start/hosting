package hostctl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// sshKeyPath returns the path to the SSH private key.
// Uses SSH_KEY_PATH env var or defaults to ~/.ssh/id_rsa.
func sshKeyPath() string {
	if p := os.Getenv("SSH_KEY_PATH"); p != "" {
		return p
	}
	return os.ExpandEnv("${HOME}/.ssh/id_rsa")
}

// sshPublicKeyContent reads the SSH public key content.
// Uses SSH_PUBLIC_KEY env var, or reads {sshKeyPath()}.pub.
func sshPublicKeyContent() (string, error) {
	if k := os.Getenv("SSH_PUBLIC_KEY"); k != "" {
		return strings.TrimSpace(k), nil
	}
	pubPath := sshKeyPath() + ".pub"
	data, err := os.ReadFile(pubPath)
	if err != nil {
		return "", fmt.Errorf("read SSH public key %s: %w", pubPath, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// sshExec runs a command on a remote host via SSH as the ubuntu user.
// Times out after 60 seconds.
func sshExec(ip, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-i", sshKeyPath(),
		"ubuntu@" + ip,
		cmd,
	}
	out, err := exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("ssh %s: timed out after 60s running %q", ip, cmd)
	}
	if err != nil {
		return "", fmt.Errorf("ssh %s %q: %w\n%s", ip, cmd, err, string(out))
	}
	return string(out), nil
}

// scpFile copies a local file to a remote host via SCP.
// Times out after 60 seconds.
func scpFile(ip, localPath, remotePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-i", sshKeyPath(),
		localPath,
		"ubuntu@" + ip + ":" + remotePath,
	}
	out, err := exec.CommandContext(ctx, "scp", args...).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("scp to %s:%s: timed out after 60s", ip, remotePath)
	}
	if err != nil {
		return fmt.Errorf("scp to %s:%s: %w\n%s", ip, remotePath, err, string(out))
	}
	return nil
}

// findNodeIPsByRole returns all node IPs for the given role in a cluster,
// sorted by shard_index.
func findNodeIPsByRole(client *Client, clusterID, role string) ([]string, error) {
	// Find the shard ID for this role.
	shardResp, err := client.Get(fmt.Sprintf("/clusters/%s/shards", clusterID))
	if err != nil {
		return nil, fmt.Errorf("list shards: %w", err)
	}
	shardItems, err := shardResp.Items()
	if err != nil {
		return nil, fmt.Errorf("parse shards: %w", err)
	}
	var shards []map[string]any
	if err := json.Unmarshal(shardItems, &shards); err != nil {
		return nil, fmt.Errorf("unmarshal shards: %w", err)
	}

	var shardID string
	for _, s := range shards {
		if r, _ := s["role"].(string); r == role {
			shardID, _ = s["id"].(string)
			break
		}
	}
	if shardID == "" {
		return nil, fmt.Errorf("no shard with role %q in cluster %s", role, clusterID)
	}

	// List all nodes in the cluster, filter by shard_id.
	nodeResp, err := client.Get(fmt.Sprintf("/clusters/%s/nodes", clusterID))
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	nodeItems, err := nodeResp.Items()
	if err != nil {
		return nil, fmt.Errorf("parse nodes: %w", err)
	}
	var nodes []map[string]any
	if err := json.Unmarshal(nodeItems, &nodes); err != nil {
		return nil, fmt.Errorf("unmarshal nodes: %w", err)
	}

	type nodeEntry struct {
		ip         string
		shardIndex int
	}
	var entries []nodeEntry
	for _, n := range nodes {
		shards, _ := n["shards"].([]any)
		for _, s := range shards {
			sm, _ := s.(map[string]any)
			if sid, _ := sm["shard_id"].(string); sid == shardID {
				if ip, ok := n["ip_address"].(string); ok && ip != "" {
					if idx := strings.Index(ip, "/"); idx != -1 {
						ip = ip[:idx]
					}
					si := 0
					if v, ok := sm["shard_index"].(float64); ok {
						si = int(v)
					}
					entries = append(entries, nodeEntry{ip: ip, shardIndex: si})
				}
				break
			}
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].shardIndex < entries[j].shardIndex
	})

	var ips []string
	for _, e := range entries {
		ips = append(ips, e.ip)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("no nodes with role %q found in cluster %s", role, clusterID)
	}
	return ips, nil
}
