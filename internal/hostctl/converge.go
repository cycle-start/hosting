package hostctl

import "fmt"

// ConvergeShard triggers shard convergence via the API.
func ConvergeShard(apiURL, shardID string) error {
	client := NewClient(apiURL)
	resp, err := client.Post(fmt.Sprintf("/api/v1/shards/%s/converge", shardID), nil)
	if err != nil {
		return err
	}
	fmt.Printf("Convergence started (status %d): %s\n", resp.StatusCode, string(resp.Body))
	return nil
}
