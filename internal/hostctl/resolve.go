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

func (c *Client) FindBrandByName(name string) (string, error) {
	return c.findByName("/brands", name)
}

func (c *Client) findByName(path, name string) (string, error) {
	resp, err := c.Get(path)
	if err != nil {
		return "", err
	}

	items, err := resp.Items()
	if err != nil {
		return "", fmt.Errorf("parse resources from %s: %w", path, err)
	}

	var resources []namedResource
	if err := json.Unmarshal(items, &resources); err != nil {
		return "", fmt.Errorf("parse resources from %s: %w", path, err)
	}

	for _, r := range resources {
		if r.Name == name {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("%q not found at %s", name, path)
}
