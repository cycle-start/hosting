package workflow

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// CreateDaemonWorkflow provisions a daemon on all nodes in the tenant's shard.
func CreateDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Fetch daemon, webroot, tenant, and nodes in one activity.
	var daemonCtx activity.DaemonContext
	err = workflow.ExecuteActivity(ctx, "GetDaemonContext", daemonID).Get(ctx, &daemonCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "daemons", daemonID, err)
		return err
	}

	if daemonCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", daemonCtx.Daemon.TenantID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noShardErr)
		return noShardErr
	}

	createParams := activity.CreateDaemonParams{
		ID:           daemonCtx.Daemon.ID,
		TenantName:   daemonCtx.Tenant.Name,
		WebrootName:  daemonCtx.Webroot.Name,
		Name:         daemonCtx.Daemon.Name,
		Command:      daemonCtx.Daemon.Command,
		ProxyPort:    daemonCtx.Daemon.ProxyPort,
		NumProcs:     daemonCtx.Daemon.NumProcs,
		StopSignal:   daemonCtx.Daemon.StopSignal,
		StopWaitSecs: daemonCtx.Daemon.StopWaitSecs,
		MaxMemoryMB:  daemonCtx.Daemon.MaxMemoryMB,
		Environment:  daemonCtx.Daemon.Environment,
	}

	// Write config and start on all nodes.
	var errs []string
	for _, node := range daemonCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "CreateDaemonConfig", createParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: create config: %v", node.ID, err))
		}
	}

	// If the daemon has a proxy_path, regenerate nginx on all nodes.
	if daemonCtx.Daemon.ProxyPath != nil {
		nginxErrs := regenerateWebrootNginxOnNodes(ctx, daemonCtx.Webroot, daemonCtx.Tenant, daemonCtx.Nodes)
		errs = append(errs, nginxErrs...)
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "daemons",
			ID:            daemonID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("create daemon failed: %s", msg)
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// UpdateDaemonWorkflow updates the daemon configuration on all nodes.
func UpdateDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var daemonCtx activity.DaemonContext
	err = workflow.ExecuteActivity(ctx, "GetDaemonContext", daemonID).Get(ctx, &daemonCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "daemons", daemonID, err)
		return err
	}

	if daemonCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", daemonCtx.Daemon.TenantID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noShardErr)
		return noShardErr
	}

	updateParams := activity.CreateDaemonParams{
		ID:           daemonCtx.Daemon.ID,
		TenantName:   daemonCtx.Tenant.Name,
		WebrootName:  daemonCtx.Webroot.Name,
		Name:         daemonCtx.Daemon.Name,
		Command:      daemonCtx.Daemon.Command,
		ProxyPort:    daemonCtx.Daemon.ProxyPort,
		NumProcs:     daemonCtx.Daemon.NumProcs,
		StopSignal:   daemonCtx.Daemon.StopSignal,
		StopWaitSecs: daemonCtx.Daemon.StopWaitSecs,
		MaxMemoryMB:  daemonCtx.Daemon.MaxMemoryMB,
		Environment:  daemonCtx.Daemon.Environment,
	}

	var errs []string
	for _, node := range daemonCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "UpdateDaemonConfig", updateParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: update config: %v", node.ID, err))
		}
	}

	// Always regenerate nginx in case proxy_path changed.
	nginxErrs := regenerateWebrootNginxOnNodes(ctx, daemonCtx.Webroot, daemonCtx.Tenant, daemonCtx.Nodes)
	errs = append(errs, nginxErrs...)

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "daemons",
			ID:            daemonID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("update daemon failed: %s", msg)
	}

	// Set status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DeleteDaemonWorkflow removes daemon config from all nodes.
func DeleteDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var daemonCtx activity.DaemonContext
	err = workflow.ExecuteActivity(ctx, "GetDaemonContext", daemonID).Get(ctx, &daemonCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "daemons", daemonID, err)
		return err
	}

	if daemonCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", daemonCtx.Daemon.TenantID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noShardErr)
		return noShardErr
	}

	deleteParams := activity.DeleteDaemonParams{
		ID:          daemonCtx.Daemon.ID,
		TenantName:  daemonCtx.Tenant.Name,
		WebrootName: daemonCtx.Webroot.Name,
		Name:        daemonCtx.Daemon.Name,
	}

	var errs []string
	for _, node := range daemonCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "DeleteDaemonConfig", deleteParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: delete config: %v", node.ID, err))
		}
	}

	// Regenerate nginx to remove the proxy location.
	if daemonCtx.Daemon.ProxyPath != nil {
		nginxErrs := regenerateWebrootNginxOnNodes(ctx, daemonCtx.Webroot, daemonCtx.Tenant, daemonCtx.Nodes)
		errs = append(errs, nginxErrs...)
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "daemons",
			ID:            daemonID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("delete daemon failed: %s", msg)
	}

	// Set status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}

// EnableDaemonWorkflow starts the daemon on all nodes.
func EnableDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var daemonCtx activity.DaemonContext
	err = workflow.ExecuteActivity(ctx, "GetDaemonContext", daemonID).Get(ctx, &daemonCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "daemons", daemonID, err)
		return err
	}

	if daemonCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", daemonCtx.Daemon.TenantID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noShardErr)
		return noShardErr
	}

	enableParams := activity.DaemonEnableParams{
		ID:          daemonCtx.Daemon.ID,
		TenantName:  daemonCtx.Tenant.Name,
		WebrootName: daemonCtx.Webroot.Name,
		Name:        daemonCtx.Daemon.Name,
	}

	var errs []string
	for _, node := range daemonCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "EnableDaemon", enableParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: %v", node.ID, err))
		}
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		_ = setResourceFailed(ctx, "daemons", daemonID, fmt.Errorf("%s", msg))
		return fmt.Errorf("enable daemon failed: %s", msg)
	}

	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DisableDaemonWorkflow stops the daemon on all nodes.
func DisableDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	var daemonCtx activity.DaemonContext
	err = workflow.ExecuteActivity(ctx, "GetDaemonContext", daemonID).Get(ctx, &daemonCtx)
	if err != nil {
		_ = setResourceFailed(ctx, "daemons", daemonID, err)
		return err
	}

	if daemonCtx.Tenant.ShardID == nil {
		noShardErr := fmt.Errorf("tenant %s has no shard assigned", daemonCtx.Daemon.TenantID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noShardErr)
		return noShardErr
	}

	disableParams := activity.DaemonEnableParams{
		ID:          daemonCtx.Daemon.ID,
		TenantName:  daemonCtx.Tenant.Name,
		WebrootName: daemonCtx.Webroot.Name,
		Name:        daemonCtx.Daemon.Name,
	}

	var errs []string
	for _, node := range daemonCtx.Nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "DisableDaemon", disableParams).Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("node %s: %v", node.ID, err))
		}
	}

	if len(errs) > 0 {
		msg := strings.Join(errs, "; ")
		if len(msg) > 4000 {
			msg = msg[:4000]
		}
		_ = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
			Table:         "daemons",
			ID:            daemonID,
			Status:        model.StatusFailed,
			StatusMessage: &msg,
		}).Get(ctx, nil)
		return fmt.Errorf("disable daemon failed: %s", msg)
	}

	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// regenerateWebrootNginxOnNodes fetches daemons and FQDNs for a webroot,
// regenerates the nginx config with daemon proxy locations on all nodes, and reloads nginx.
func regenerateWebrootNginxOnNodes(ctx workflow.Context, webroot model.Webroot, tenant model.Tenant, nodes []model.Node) []string {
	var errs []string

	// Fetch active daemons for this webroot to build proxy locations.
	var daemons []model.Daemon
	err := workflow.ExecuteActivity(ctx, "ListDaemonsByWebroot", webroot.ID).Get(ctx, &daemons)
	if err != nil {
		return []string{fmt.Sprintf("list daemons for webroot %s: %v", webroot.ID, err)}
	}

	// Fetch FQDNs for this webroot.
	var fqdns []model.FQDN
	err = workflow.ExecuteActivity(ctx, "GetFQDNsByWebrootID", webroot.ID).Get(ctx, &fqdns)
	if err != nil {
		return []string{fmt.Sprintf("get fqdns for webroot %s: %v", webroot.ID, err)}
	}

	var fqdnParams []activity.FQDNParam
	for _, f := range fqdns {
		if f.Status != model.StatusDeleted {
			fqdnParams = append(fqdnParams, activity.FQDNParam{
				FQDN:       f.FQDN,
				WebrootID:  f.WebrootID,
				SSLEnabled: f.SSLEnabled,
			})
		}
	}

	// Build daemon proxy info for active daemons with proxy_path.
	var daemonProxies []activity.DaemonProxyInfo
	for _, d := range daemons {
		if d.Status == model.StatusActive && d.ProxyPath != nil && d.ProxyPort != nil {
			daemonProxies = append(daemonProxies, activity.DaemonProxyInfo{
				ProxyPath: *d.ProxyPath,
				Port:      *d.ProxyPort,
			})
		}
	}

	// Regenerate nginx on each node by calling UpdateWebroot which handles nginx config.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UpdateWebroot", activity.UpdateWebrootParams{
			ID:             webroot.ID,
			TenantName:     tenant.Name,
			Name:           webroot.Name,
			Runtime:        webroot.Runtime,
			RuntimeVersion: webroot.RuntimeVersion,
			RuntimeConfig:  string(webroot.RuntimeConfig),
			PublicFolder:   webroot.PublicFolder,
			FQDNs:          fqdnParams,
			Daemons:        daemonProxies,
		}).Get(ctx, nil)
		if err != nil {
			errs = append(errs, fmt.Sprintf("regenerate nginx on node %s: %v", node.ID, err))
		}
	}

	// Reload nginx on all nodes.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		if err := workflow.ExecuteActivity(nodeCtx, "ReloadNginx").Get(ctx, nil); err != nil {
			errs = append(errs, fmt.Sprintf("reload nginx on node %s: %v", node.ID, err))
		}
	}

	return errs
}
