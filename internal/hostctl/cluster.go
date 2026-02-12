package hostctl

import (
	"encoding/json"
	"fmt"
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

	client := NewClient(cfg.APIURL)

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
	} else {
		fmt.Printf("Cluster %q: %s (exists)\n", cfg.Cluster.Name, clusterID)
	}

	// 3. Register node profiles
	for _, p := range cfg.NodeProfiles {
		profileID, err := findOrCreateNodeProfile(client, p)
		if err != nil {
			return fmt.Errorf("node profile %q: %w", p.Name, err)
		}
		fmt.Printf("Node profile %q (role=%s): %s\n", p.Name, p.Role, profileID)
	}

	// 4. Register host machines
	for _, h := range cfg.Hosts {
		hostID, err := findOrCreateHost(client, clusterID, h)
		if err != nil {
			return fmt.Errorf("host %q: %w", h.Hostname, err)
		}
		fmt.Printf("Host %q: %s\n", h.Hostname, hostID)
	}

	// 5. Provision cluster
	fmt.Println("Provisioning cluster...")
	_, err = client.Post(fmt.Sprintf("/clusters/%s/provision", clusterID), nil)
	if err != nil {
		return fmt.Errorf("provision cluster: %w", err)
	}

	// 6. Wait for cluster to become active
	fmt.Printf("Waiting for cluster (timeout: %s)...\n", timeout)
	if err := client.WaitForStatus(fmt.Sprintf("/clusters/%s", clusterID), "active", timeout); err != nil {
		return fmt.Errorf("wait for cluster: %w", err)
	}
	fmt.Println("Cluster is active!")

	// 7. Print summary
	return printClusterSummary(client, clusterID)
}

func findOrCreateRegion(client *Client, def RegionDef) (string, error) {
	id, err := client.FindRegionByName(def.Name)
	if err == nil {
		return id, nil
	}

	regionID := def.ID
	if regionID == "" {
		regionID = def.Name
	}

	resp, err := client.Post("/regions", map[string]any{
		"id":   regionID,
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
		if def.Spec.Infrastructure.ServiceDB {
			infra["service_db"] = true
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

func findOrCreateHost(client *Client, clusterID string, def HostDef) (string, error) {
	id, err := client.FindHostByHostname(clusterID, def.Hostname)
	if err == nil {
		return id, nil
	}

	body := map[string]any{
		"hostname":    def.Hostname,
		"ip_address":  def.IPAddress,
		"docker_host": def.DockerHost,
	}
	if def.Capacity.MaxNodes > 0 {
		body["capacity"] = map[string]any{
			"max_nodes": def.Capacity.MaxNodes,
		}
	}
	if len(def.Roles) > 0 {
		body["roles"] = def.Roles
	}

	resp, err := client.Post(fmt.Sprintf("/clusters/%s/hosts", clusterID), body)
	if err != nil {
		return "", err
	}

	return extractID(resp)
}

func findOrCreateNodeProfile(client *Client, def NodeProfileDef) (string, error) {
	id, err := client.findByName("/node-profiles", def.Name)
	if err == nil {
		return id, nil
	}

	body := map[string]any{
		"name":  def.Name,
		"role":  def.Role,
		"image": def.Image,
	}
	if def.Env != nil {
		body["env"] = def.Env
	}
	if len(def.Volumes) > 0 {
		body["volumes"] = def.Volumes
	}
	if len(def.Ports) > 0 {
		ports := make([]map[string]int, len(def.Ports))
		for i, p := range def.Ports {
			ports[i] = map[string]int{"host": p.Host, "container": p.Container}
		}
		body["ports"] = ports
	}
	if def.Resources != (ResourcesDef{}) {
		body["resources"] = map[string]any{
			"memory_mb":  def.Resources.MemoryMB,
			"cpu_shares": def.Resources.CPUShares,
		}
	}
	if def.HealthCheck != nil {
		body["health_check"] = def.HealthCheck
	}
	if def.Privileged {
		body["privileged"] = true
	}
	if def.NetworkMode != "" {
		body["network_mode"] = def.NetworkMode
	}

	resp, err := client.Post("/node-profiles", body)
	if err != nil {
		return "", err
	}

	return extractID(resp)
}

func printClusterSummary(client *Client, clusterID string) error {
	// List shards
	resp, err := client.Get(fmt.Sprintf("/clusters/%s/shards", clusterID))
	if err != nil {
		return err
	}

	var shards []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	if err := json.Unmarshal(resp.Body, &shards); err != nil {
		return fmt.Errorf("parse shards: %w", err)
	}

	fmt.Println("\nShards:")
	for _, s := range shards {
		fmt.Printf("  %-20s role=%-10s id=%s\n", s.Name, s.Role, s.ID)
	}

	// List nodes
	resp, err = client.Get(fmt.Sprintf("/clusters/%s/nodes", clusterID))
	if err != nil {
		return err
	}

	var nodes []struct {
		ID       string  `json:"id"`
		ShardID  *string `json:"shard_id"`
		Hostname string  `json:"hostname"`
		Status   string  `json:"status"`
	}
	if err := json.Unmarshal(resp.Body, &nodes); err != nil {
		return fmt.Errorf("parse nodes: %w", err)
	}

	fmt.Println("\nNodes:")
	for _, n := range nodes {
		shardStr := "<none>"
		if n.ShardID != nil {
			shardStr = *n.ShardID
		}
		fmt.Printf("  %-30s status=%-12s shard=%s id=%s\n", n.Hostname, n.Status, shardStr, n.ID)
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
