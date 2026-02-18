package model

// ProvisionSignalName is the Temporal signal name used to route workflow tasks
// to the per-tenant entity workflow.
const ProvisionSignalName = "provision"

// ProvisionTask describes a workflow to execute within the per-tenant entity
// workflow. It is sent as a signal payload to TenantProvisionWorkflow.
type ProvisionTask struct {
	WorkflowName string `json:"workflow_name"`
	WorkflowID   string `json:"workflow_id"`
	Arg          any    `json:"arg"`
}
