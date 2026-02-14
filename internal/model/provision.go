package model

// ProvisionSignalName is the signal name used by the per-tenant provisioning workflow.
const ProvisionSignalName = "provision"

// ProvisionTask represents a unit of work to be processed sequentially
// by the per-tenant provisioning workflow.
type ProvisionTask struct {
	WorkflowName string `json:"workflow_name"`
	WorkflowID   string `json:"workflow_id"`
	Arg          any    `json:"arg"`
	CallbackURL  string `json:"callback_url,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
	ResourceID   string `json:"resource_id,omitempty"`
}
