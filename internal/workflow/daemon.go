package workflow

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
)

// findDaemonNode returns the node assigned to the daemon, or nil if not found.
func findDaemonNode(dc *activity.DaemonContext) *model.Node {
	if dc.Daemon.NodeID == nil {
		return nil
	}
	for _, n := range dc.Nodes {
		if n.ID == *dc.Daemon.NodeID {
			return &n
		}
	}
	return nil
}

// computeDaemonHostIP computes the tenant's ULA address on the daemon's assigned node.
// Returns empty string if the node has no shard_index.
func computeDaemonHostIP(dc *activity.DaemonContext, node *model.Node) string {
	if node == nil || node.ShardIndex == nil {
		return ""
	}
	clusterID := node.ClusterID
	return core.ComputeTenantULA(clusterID, *node.ShardIndex, dc.Tenant.UID)
}

// CreateDaemonWorkflow provisions a daemon on its assigned node.
func CreateDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	node := findDaemonNode(&daemonCtx)
	if node == nil {
		noNodeErr := fmt.Errorf("daemon %s has no node assigned", daemonID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noNodeErr)
		return noNodeErr
	}

	createParams := activity.CreateDaemonParams{
		ID:           daemonCtx.Daemon.ID,
		NodeID:       daemonCtx.Daemon.NodeID,
		TenantName:   daemonCtx.Tenant.ID,
		WebrootName:  daemonCtx.Webroot.ID,
		Name:         daemonCtx.Daemon.ID,
		Command:      daemonCtx.Daemon.Command,
		ProxyPort:    daemonCtx.Daemon.ProxyPort,
		HostIP:       computeDaemonHostIP(&daemonCtx, node),
		NumProcs:     daemonCtx.Daemon.NumProcs,
		StopSignal:   daemonCtx.Daemon.StopSignal,
		StopWaitSecs: daemonCtx.Daemon.StopWaitSecs,
		MaxMemoryMB:  daemonCtx.Daemon.MaxMemoryMB,
		EnvFileName:  daemonCtx.Webroot.EnvFileName,
	}

	// Write config and start on the daemon's assigned node.
	var errs []string
	nodeCtx := nodeActivityCtx(ctx, node.ID)
	if err := workflow.ExecuteActivity(nodeCtx, "CreateDaemonConfig", createParams).Get(ctx, nil); err != nil {
		errs = append(errs, fmt.Sprintf("node %s: create config: %v", node.ID, err))
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

// UpdateDaemonWorkflow updates the daemon configuration on its assigned node.
func UpdateDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	node := findDaemonNode(&daemonCtx)
	if node == nil {
		noNodeErr := fmt.Errorf("daemon %s has no node assigned", daemonID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noNodeErr)
		return noNodeErr
	}

	updateParams := activity.CreateDaemonParams{
		ID:           daemonCtx.Daemon.ID,
		NodeID:       daemonCtx.Daemon.NodeID,
		TenantName:   daemonCtx.Tenant.ID,
		WebrootName:  daemonCtx.Webroot.ID,
		Name:         daemonCtx.Daemon.ID,
		Command:      daemonCtx.Daemon.Command,
		ProxyPort:    daemonCtx.Daemon.ProxyPort,
		HostIP:       computeDaemonHostIP(&daemonCtx, node),
		NumProcs:     daemonCtx.Daemon.NumProcs,
		StopSignal:   daemonCtx.Daemon.StopSignal,
		StopWaitSecs: daemonCtx.Daemon.StopWaitSecs,
		MaxMemoryMB:  daemonCtx.Daemon.MaxMemoryMB,
		EnvFileName:  daemonCtx.Webroot.EnvFileName,
	}

	// Update config on the daemon's assigned node.
	var errs []string
	nodeCtx := nodeActivityCtx(ctx, node.ID)
	if err := workflow.ExecuteActivity(nodeCtx, "UpdateDaemonConfig", updateParams).Get(ctx, nil); err != nil {
		errs = append(errs, fmt.Sprintf("node %s: update config: %v", node.ID, err))
	}

	// Always regenerate nginx on all nodes in case proxy_path changed.
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

// DeleteDaemonWorkflow removes daemon config from the daemon's assigned node.
func DeleteDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	node := findDaemonNode(&daemonCtx)
	if node == nil {
		noNodeErr := fmt.Errorf("daemon %s has no node assigned", daemonID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noNodeErr)
		return noNodeErr
	}

	deleteParams := activity.DeleteDaemonParams{
		ID:          daemonCtx.Daemon.ID,
		TenantName:  daemonCtx.Tenant.ID,
		WebrootName: daemonCtx.Webroot.ID,
		Name:        daemonCtx.Daemon.ID,
	}

	// Delete config on the daemon's assigned node.
	var errs []string
	nodeCtx := nodeActivityCtx(ctx, node.ID)
	if err := workflow.ExecuteActivity(nodeCtx, "DeleteDaemonConfig", deleteParams).Get(ctx, nil); err != nil {
		errs = append(errs, fmt.Sprintf("node %s: delete config: %v", node.ID, err))
	}

	// Regenerate nginx on all nodes to remove the proxy location.
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

// EnableDaemonWorkflow starts the daemon on its assigned node.
func EnableDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	node := findDaemonNode(&daemonCtx)
	if node == nil {
		noNodeErr := fmt.Errorf("daemon %s has no node assigned", daemonID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noNodeErr)
		return noNodeErr
	}

	enableParams := activity.DaemonEnableParams{
		ID:          daemonCtx.Daemon.ID,
		TenantName:  daemonCtx.Tenant.ID,
		WebrootName: daemonCtx.Webroot.ID,
		Name:        daemonCtx.Daemon.ID,
	}

	// Enable on the daemon's assigned node only.
	nodeCtx := nodeActivityCtx(ctx, node.ID)
	if err := workflow.ExecuteActivity(nodeCtx, "EnableDaemon", enableParams).Get(ctx, nil); err != nil {
		_ = setResourceFailed(ctx, "daemons", daemonID, err)
		return fmt.Errorf("enable daemon failed: node %s: %v", node.ID, err)
	}

	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "daemons",
		ID:     daemonID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DisableDaemonWorkflow stops the daemon on its assigned node.
func DisableDaemonWorkflow(ctx workflow.Context, daemonID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:    3,
			InitialInterval:    1 * time.Second,
			MaximumInterval:    10 * time.Second,
			BackoffCoefficient: 2.0,
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

	node := findDaemonNode(&daemonCtx)
	if node == nil {
		noNodeErr := fmt.Errorf("daemon %s has no node assigned", daemonID)
		_ = setResourceFailed(ctx, "daemons", daemonID, noNodeErr)
		return noNodeErr
	}

	disableParams := activity.DaemonEnableParams{
		ID:          daemonCtx.Daemon.ID,
		TenantName:  daemonCtx.Tenant.ID,
		WebrootName: daemonCtx.Webroot.ID,
		Name:        daemonCtx.Daemon.ID,
	}

	// Disable on the daemon's assigned node only.
	nodeCtx := nodeActivityCtx(ctx, node.ID)
	if err := workflow.ExecuteActivity(nodeCtx, "DisableDaemon", disableParams).Get(ctx, nil); err != nil {
		_ = setResourceFailed(ctx, "daemons", daemonID, err)
		return fmt.Errorf("disable daemon failed: node %s: %v", node.ID, err)
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
			var webrootID string
			if f.WebrootID != nil {
				webrootID = *f.WebrootID
			}
			fqdnParams = append(fqdnParams, activity.FQDNParam{
				FQDN:       f.FQDN,
				WebrootID:  webrootID,
				SSLEnabled: f.SSLEnabled,
			})
		}
	}

	// Build a node index map for ULA computation.
	nodeShardIndex := make(map[string]int) // node ID -> shard_index
	for _, n := range nodes {
		if n.ShardIndex != nil {
			nodeShardIndex[n.ID] = *n.ShardIndex
		}
	}

	// Determine the cluster ID from the first node.
	clusterID := ""
	if len(nodes) > 0 {
		clusterID = nodes[0].ClusterID
	}

	// Build daemon proxy info for active/provisioning daemons with proxy_path.
	// Include provisioning daemons because this function is called during CreateDaemonWorkflow
	// before the daemon's status is set to active.
	var daemonProxies []activity.DaemonProxyInfo
	for _, d := range daemons {
		if (d.Status == model.StatusActive || d.Status == model.StatusProvisioning) && d.ProxyPath != nil && d.ProxyPort != nil {
			targetIP := "127.0.0.1"
			if d.NodeID != nil {
				if idx, ok := nodeShardIndex[*d.NodeID]; ok {
					targetIP = core.ComputeTenantULA(clusterID, idx, tenant.UID)
				}
			}
			daemonProxies = append(daemonProxies, activity.DaemonProxyInfo{
				ProxyPath: *d.ProxyPath,
				Port:      *d.ProxyPort,
				TargetIP:  targetIP,
				ProxyURL:  core.FormatDaemonProxyURL(targetIP, *d.ProxyPort),
			})
		}
	}

	// Regenerate nginx on each node by calling UpdateWebroot which handles nginx config.
	for _, node := range nodes {
		nodeCtx := nodeActivityCtx(ctx, node.ID)
		err = workflow.ExecuteActivity(nodeCtx, "UpdateWebroot", activity.UpdateWebrootParams{
			ID:             webroot.ID,
			TenantName:     tenant.ID,
			Name:           webroot.ID,
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
