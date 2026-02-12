package hostctl

import (
	"encoding/json"
	"fmt"
)

type namedResource struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) FindRegionByName(name string) (string, error) {
	return c.findByName("/regions", name)
}

func (c *Client) FindClusterByName(regionID string, name string) (string, error) {
	return c.findByName(fmt.Sprintf("/regions/%s/clusters", regionID), name)
}

func (c *Client) FindShardByName(clusterID string, name string) (string, error) {
	return c.findByName(fmt.Sprintf("/clusters/%s/shards", clusterID), name)
}

func (c *Client) FindTenantByName(name string) (string, error) {
	return c.findByName("/tenants", name)
}

func (c *Client) FindHostByHostname(clusterID string, hostname string) (string, error) {
	resp, err := c.Get(fmt.Sprintf("/clusters/%s/hosts", clusterID))
	if err != nil {
		return "", err
	}

	var hosts []struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
	}
	if err := json.Unmarshal(resp.Body, &hosts); err != nil {
		return "", fmt.Errorf("parse hosts: %w", err)
	}

	for _, h := range hosts {
		if h.Hostname == hostname {
			return h.ID, nil
		}
	}
	return "", fmt.Errorf("host %q not found in cluster %s", hostname, clusterID)
}

func (c *Client) findByName(path, name string) (string, error) {
	resp, err := c.Get(path)
	if err != nil {
		return "", err
	}

	var resources []namedResource
	if err := json.Unmarshal(resp.Body, &resources); err != nil {
		return "", fmt.Errorf("parse resources from %s: %w", path, err)
	}

	for _, r := range resources {
		if r.Name == name {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("%q not found at %s", name, path)
}
