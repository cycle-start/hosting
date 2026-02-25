package hostctl

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func ClusterApply(configPath string, timeout time.Duration) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var cfg ClusterConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("HOSTING_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("no API key: set api_key in config or HOSTING_API_KEY env var")
	}
	client := NewClient(cfg.APIURL, apiKey)

	// Wait for API to be reachable before proceeding.
	fmt.Printf("Waiting for API at %s...\n", cfg.APIURL)
	probeClient := &Client{
		BaseURL:    cfg.APIURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
	}
	deadline := time.Now().Add(timeout)
	for {
		_, err := probeClient.Get("/regions")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("API not reachable after %s: %w", timeout, err)
		}
		time.Sleep(3 * time.Second)
	}

	// 1. Find or create region
	regionID, err := findOrCreateRegion(client, cfg.Region)
	if err != nil {
		return fmt.Errorf("region: %w", err)
	}
	fmt.Printf("Region %q: %s\n", cfg.Region.Name, regionID)

	// 2. Find or create cluster
	clusterID, created, err := findOrCreateCluster(client, regionID, cfg.Cluster)
	if err != nil {
		return fmt.Errorf("cluster: %w", err)
	}
	if created {
		fmt.Printf("Cluster %q: %s (created)\n", cfg.Cluster.Name, clusterID)

		// 2b. Create LB addresses for the new cluster
		for _, lb := range cfg.Cluster.LBAddresses {
			_, err := client.Post(fmt.Sprintf("/clusters/%s/lb-addresses", clusterID), map[string]any{
				"address": lb.Address,
				"label":   lb.Label,
			})
			if err != nil {
				return fmt.Errorf("create LB address %q: %w", lb.Address, err)
			}
			fmt.Printf("  LB address %q: created\n", lb.Address)
		}

		// 2c. Create shards from spec
		for _, s := range cfg.Cluster.Spec.Shards {
			body := map[string]any{
				"name": s.Name,
				"role": s.Role,
			}
			if s.LBBackend != "" {
				body["lb_backend"] = s.LBBackend
			}
			_, err := client.Post(fmt.Sprintf("/clusters/%s/shards", clusterID), body)
			if err != nil {
				return fmt.Errorf("create shard %q: %w", s.Name, err)
			}
			fmt.Printf("  Shard %q (role=%s): created\n", s.Name, s.Role)
		}
	} else {
		fmt.Printf("Cluster %q: %s (exists)\n", cfg.Cluster.Name, clusterID)

		// Update config and spec on existing cluster.
		if err := updateCluster(client, clusterID, cfg.Cluster); err != nil {
			return fmt.Errorf("update cluster: %w", err)
		}
	}

	// 3. Apply nodes (create via API with shard assignment, converge)
	if err := applyNodes(client, clusterID, cfg.Cluster); err != nil {
		return err
	}

	// 4. Set cluster status to active
	_, err = client.Put(fmt.Sprintf("/clusters/%s", clusterID), map[string]any{
		"status": "active",
	})
	if err != nil {
		return fmt.Errorf("set cluster active: %w", err)
	}
	fmt.Printf("Cluster %q: active\n", cfg.Cluster.Name)

	// 5. Seed cluster runtimes
	for _, rr := range cfg.ClusterRuntimes {
		fmt.Printf("Adding cluster runtime %s %s...\n", rr.Runtime, rr.Version)
		_, err := client.Post(fmt.Sprintf("/clusters/%s/runtimes", clusterID), map[string]any{
			"runtime": rr.Runtime,
			"version": rr.Version,
		})
		if err != nil {
			return fmt.Errorf("add cluster runtime %s %s: %w", rr.Runtime, rr.Version, err)
		}
	}

	return nil
}

func applyNodes(client *Client, clusterID string, def ClusterDef) error {
	// Build shard name -> ID map and shard name -> role map.
	resp, err := client.Get(fmt.Sprintf("/clusters/%s/shards", clusterID))
	if err != nil {
		return fmt.Errorf("list shards: %w", err)
	}
	items, err := resp.Items()
	if err != nil {
		return fmt.Errorf("parse shards: %w", err)
	}
	var shards []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(items, &shards); err != nil {
		return fmt.Errorf("parse shards: %w", err)
	}
	shardNameToID := make(map[string]string)
	shardRoleMap := make(map[string]string)
	for _, s := range shards {
		shardNameToID[s.Name] = s.ID
		shardRoleMap[s.Name] = s.Role
	}

	// For each node, ensure it exists with the correct shard assignment(s).
	for _, node := range def.Nodes {
		// Merge ShardName into ShardNames for backward compatibility.
		shardNames := node.ShardNames
		if len(shardNames) == 0 && node.ShardName != "" {
			shardNames = []string{node.ShardName}
		}

		// Resolve shard names to IDs and collect roles.
		var shardIDs []string
		roles := make(map[string]bool)
		for _, name := range shardNames {
			role, ok := shardRoleMap[name]
			if !ok {
				return fmt.Errorf("shard %q not found for node %s", name, node.ID)
			}
			shardIDs = append(shardIDs, shardNameToID[name])
			roles[role] = true
		}
		roleList := make([]string, 0, len(roles))
		for r := range roles {
			roleList = append(roleList, r)
		}

		hostname := node.Hostname
		if hostname == "" {
			hostname = node.ID
		}

		// Try to find existing node by ID.
		existingResp, err := client.Get(fmt.Sprintf("/nodes/%s", node.ID))
		if err == nil {
			// Check if hostname needs updating.
			var existing struct {
				Hostname string `json:"hostname"`
			}
			if json.Unmarshal(existingResp.Body, &existing) == nil && existing.Hostname != hostname {
				_, err := client.Put(fmt.Sprintf("/nodes/%s", node.ID), map[string]any{
					"hostname": hostname,
				})
				if err != nil {
					fmt.Printf("Node %s: warning: failed to update hostname: %v\n", hostname, err)
				} else {
					fmt.Printf("Node %s: updated hostname (was %q)\n", hostname, existing.Hostname)
				}
			} else {
				fmt.Printf("Node %s: exists (shards=%v)\n", hostname, shardNames)
			}
			continue
		}

		// Create node with pre-assigned ID and shard(s).
		_, err = client.Post(fmt.Sprintf("/clusters/%s/nodes", clusterID), map[string]any{
			"id":         node.ID,
			"hostname":   hostname,
			"ip_address": node.IPAddress,
			"shard_ids":  shardIDs,
			"roles":      roleList,
		})
		if err != nil {
			return fmt.Errorf("create node %s: %w", hostname, err)
		}
		fmt.Printf("Node %s: created (shards=%v, ip=%s)\n", hostname, shardNames, node.IPAddress)
	}

	// Trigger convergence for each shard that has nodes.
	convergedShards := make(map[string]bool)
	for _, node := range def.Nodes {
		shardNames := node.ShardNames
		if len(shardNames) == 0 && node.ShardName != "" {
			shardNames = []string{node.ShardName}
		}
		for _, name := range shardNames {
			if convergedShards[name] {
				continue
			}
			convergedShards[name] = true

			shardID := shardNameToID[name]
			fmt.Printf("Converging shard %q...\n", name)
			_, err := client.Post(fmt.Sprintf("/shards/%s/converge", shardID), nil)
			if err != nil {
				fmt.Printf("  Warning: convergence failed for shard %q: %v\n", name, err)
			}
		}
	}

	fmt.Println("Nodes applied!")
	return printClusterSummary(client, clusterID)
}

func findOrCreateRegion(client *Client, def RegionDef) (string, error) {
	id, err := client.FindRegionByName(def.Name)
	if err == nil {
		return id, nil
	}

	resp, err := client.Post("/regions", map[string]any{
		"name": def.Name,
	})
	if err != nil {
		return "", err
	}

	return extractID(resp)
}

func findOrCreateCluster(client *Client, regionID string, def ClusterDef) (string, bool, error) {
	id, err := client.FindClusterByName(regionID, def.Name)
	if err == nil {
		return id, false, nil
	}

	body := map[string]any{
		"name": def.Name,
	}
	if def.Config != nil {
		body["config"] = def.Config
	}
	if len(def.Spec.Shards) > 0 || def.Spec.Infrastructure != (InfrastructureSpecDef{}) {
		spec := map[string]any{}
		if len(def.Spec.Shards) > 0 {
			shards := make([]map[string]any, len(def.Spec.Shards))
			for i, s := range def.Spec.Shards {
				shards[i] = map[string]any{
					"name":       s.Name,
					"role":       s.Role,
					"node_count": s.NodeCount,
				}
			}
			spec["shards"] = shards
		}
		infra := map[string]any{}
		if def.Spec.Infrastructure.HAProxy {
			infra["haproxy"] = true
		}
		if def.Spec.Infrastructure.PowerDNS {
			infra["powerdns"] = true
		}
		if def.Spec.Infrastructure.Valkey {
			infra["valkey"] = true
		}
		if len(infra) > 0 {
			spec["infrastructure"] = infra
		}
		body["spec"] = spec
	}

	resp, err := client.Post(fmt.Sprintf("/regions/%s/clusters", regionID), body)
	if err != nil {
		return "", false, err
	}

	id, err = extractID(resp)
	return id, true, err
}

func updateCluster(client *Client, clusterID string, def ClusterDef) error {
	body := map[string]any{}
	if def.Config != nil {
		body["config"] = def.Config
	}
	if len(def.Spec.Shards) > 0 || def.Spec.Infrastructure != (InfrastructureSpecDef{}) {
		spec := map[string]any{}
		if len(def.Spec.Shards) > 0 {
			shards := make([]map[string]any, len(def.Spec.Shards))
			for i, s := range def.Spec.Shards {
				shard := map[string]any{
					"name":       s.Name,
					"role":       s.Role,
					"node_count": s.NodeCount,
				}
				if s.LBBackend != "" {
					shard["lb_backend"] = s.LBBackend
				}
				shards[i] = shard
			}
			spec["shards"] = shards
		}
		body["spec"] = spec
	}
	if len(body) == 0 {
		return nil
	}

	_, err := client.Put(fmt.Sprintf("/clusters/%s", clusterID), body)
	if err != nil {
		return err
	}
	fmt.Printf("  Config/spec updated\n")
	return nil
}

func printClusterSummary(client *Client, clusterID string) error {
	// List shards
	resp, err := client.Get(fmt.Sprintf("/clusters/%s/shards", clusterID))
	if err != nil {
		return err
	}

	shardItems, err := resp.Items()
	if err != nil {
		return fmt.Errorf("parse shards: %w", err)
	}
	var shards []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(shardItems, &shards); err != nil {
		return fmt.Errorf("parse shards: %w", err)
	}

	fmt.Println("\nShards:")
	for _, s := range shards {
		fmt.Printf("  %-20s role=%s  id=%s\n", s.Name, s.Role, s.ID)
	}

	// List nodes
	resp, err = client.Get(fmt.Sprintf("/clusters/%s/nodes", clusterID))
	if err != nil {
		return err
	}

	nodeItems, err := resp.Items()
	if err != nil {
		return fmt.Errorf("parse nodes: %w", err)
	}
	var nodes []struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
		Status   string `json:"status"`
		Shards   []struct {
			ShardID   string `json:"shard_id"`
			ShardRole string `json:"shard_role"`
		} `json:"shards"`
	}
	if err := json.Unmarshal(nodeItems, &nodes); err != nil {
		return fmt.Errorf("parse nodes: %w", err)
	}

	fmt.Println("\nNodes:")
	for _, n := range nodes {
		shardStr := "<none>"
		if len(n.Shards) > 0 {
			roles := make([]string, len(n.Shards))
			for i, s := range n.Shards {
				roles[i] = s.ShardRole
			}
			shardStr = fmt.Sprintf("%v", roles)
		}
		fmt.Printf("  %-30s status=%-12s shards=%s id=%s\n", n.Hostname, n.Status, shardStr, n.ID)
	}

	return nil
}

func extractID(resp *Response) (string, error) {
	var resource struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &resource); err != nil {
		return "", fmt.Errorf("parse response ID: %w", err)
	}
	return resource.ID, nil
}
