package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// UpdateServiceHostnamesWorkflow looks up a tenant's service hostnames
// and creates/updates DNS records for each enabled service.
func UpdateServiceHostnamesWorkflow(ctx workflow.Context, tenantID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Look up the tenant.
	var tenant model.Tenant
	err := workflow.ExecuteActivity(ctx, "GetTenantByID", tenantID).Get(ctx, &tenant)
	if err != nil {
		return err
	}

	// Get the base hostname from platform config.
	var baseHostname string
	err = workflow.ExecuteActivity(ctx, "GetPlatformConfig", "base_hostname").Get(ctx, &baseHostname)
	if err != nil {
		return err
	}

	// Look up tenant services.
	var services []model.TenantService
	err = workflow.ExecuteActivity(ctx, "GetTenantServicesByTenantID", tenantID).Get(ctx, &services)
	if err != nil {
		return err
	}

	if len(services) == 0 {
		return nil
	}

	// Look up node IPs for each service.
	entries := make([]activity.ServiceHostnameEntry, 0, len(services))
	for _, svc := range services {
		var nodes []model.Node
		err = workflow.ExecuteActivity(ctx, "GetNodesByClusterAndRole", tenant.ClusterID, svc.Service).Get(ctx, &nodes)
		if err != nil {
			return err
		}

		if len(nodes) == 0 {
			continue
		}

		// Use the first matching node's IPs.
		node := nodes[0]
		entry := activity.ServiceHostnameEntry{Service: svc.Service}
		if node.IPAddress != nil {
			entry.IP = *node.IPAddress
		}
		if node.IP6Address != nil {
			entry.IP6 = *node.IP6Address
		}
		entries = append(entries, entry)
	}

	if len(entries) == 0 {
		return nil
	}

	// Create DNS records for service hostnames.
	err = workflow.ExecuteActivity(ctx, "CreateServiceHostnameRecords", activity.ServiceHostnameParams{
		BaseHostname: baseHostname,
		TenantName:   tenant.ID,
		Services:     entries,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
