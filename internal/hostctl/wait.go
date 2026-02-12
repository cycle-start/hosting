package hostctl

import (
	"encoding/json"
	"fmt"
	"time"
)

func (c *Client) WaitForStatus(path string, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s to reach status %q", path, target)
		}

		resp, err := c.Get(path)
		if err != nil {
			return fmt.Errorf("poll %s: %w", path, err)
		}

		var resource struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(resp.Body, &resource); err != nil {
			return fmt.Errorf("parse status from %s: %w", path, err)
		}

		if resource.Status == target {
			return nil
		}
		if resource.Status == "failed" {
			return fmt.Errorf("%s reached status \"failed\"", path)
		}

		time.Sleep(2 * time.Second)
	}
}
