package hostctl

import (
	"fmt"
)

// AwaitWorkflow blocks until the specified Temporal workflow completes.
// Uses the GET /workflows/{id}/await endpoint which blocks server-side.
func (c *Client) AwaitWorkflow(workflowID string) error {
	_, err := c.Get(fmt.Sprintf("/workflows/%s/await", workflowID))
	if err != nil {
		return fmt.Errorf("await workflow %s: %w", workflowID, err)
	}
	return nil
}
