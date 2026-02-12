package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

// ProvisionClusterWorkflow bootstraps all infrastructure and shards for a cluster.
func ProvisionClusterWorkflow(ctx workflow.Context, clusterID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set cluster status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "clusters",
		ID:     clusterID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the cluster.
	var cluster model.Cluster
	err = workflow.ExecuteActivity(ctx, "GetClusterByID", clusterID).Get(ctx, &cluster)
	if err != nil {
		_ = setResourceFailed(ctx, "clusters", clusterID)
		return err
	}

	// Parse the cluster spec.
	var spec model.ClusterSpec
	if err := json.Unmarshal(cluster.Spec, &spec); err != nil {
		_ = setResourceFailed(ctx, "clusters", clusterID)
		return fmt.Errorf("parse cluster spec: %w", err)
	}

	// Parse cluster config for docker_network.
	var clusterConfig struct {
		DockerNetwork string `json:"docker_network"`
	}
	_ = json.Unmarshal(cluster.Config, &clusterConfig)

	// List host machines in the cluster.
	var hosts []model.HostMachine
	err = workflow.ExecuteActivity(ctx, "ListHostMachinesByCluster", clusterID).Get(ctx, &hosts)
	if err != nil {
		_ = setResourceFailed(ctx, "clusters", clusterID)
		return err
	}

	if len(hosts) == 0 {
		_ = setResourceFailed(ctx, "clusters", clusterID)
		return fmt.Errorf("no host machines found for cluster %s", clusterID)
	}

	// Validate host reachability.
	for _, host := range hosts {
		err = workflow.ExecuteActivity(ctx, "ValidateHostReachable", activity.ValidateHostReachableParams{
			Host: host,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "clusters", clusterID)
			return fmt.Errorf("host %s not reachable: %w", host.Hostname, err)
		}
	}

	// Deploy infrastructure services.
	type infraDef struct {
		serviceType string
		image       string
		name        string
	}
	var infraServices []infraDef
	if spec.Infrastructure.HAProxy {
		infraServices = append(infraServices, infraDef{
			serviceType: model.InfraServiceHAProxy,
			image:       "haproxy:2.9-alpine",
			name:        fmt.Sprintf("haproxy-%s", clusterID[:8]),
		})
	}
	if spec.Infrastructure.ServiceDB {
		infraServices = append(infraServices, infraDef{
			serviceType: model.InfraServiceServiceDB,
			image:       "postgres:16",
			name:        fmt.Sprintf("service-db-%s", clusterID[:8]),
		})
	}
	if spec.Infrastructure.Valkey {
		infraServices = append(infraServices, infraDef{
			serviceType: model.InfraServiceValkey,
			image:       "valkey/valkey:8",
			name:        fmt.Sprintf("valkey-%s", clusterID[:8]),
		})
	}

	for _, infra := range infraServices {
		// Select a host for this infra service.
		var host model.HostMachine
		err = workflow.ExecuteActivity(ctx, "SelectHostForInfra", activity.SelectHostForInfraParams{
			ClusterID:   clusterID,
			ServiceType: infra.serviceType,
		}).Get(ctx, &host)
		if err != nil {
			_ = setResourceFailed(ctx, "clusters", clusterID)
			return err
		}

		// Pull the image.
		var pullResult activity.PullImageResult
		err = workflow.ExecuteActivity(ctx, "PullImage", activity.PullImageParams{
			Host:  host,
			Image: infra.image,
		}).Get(ctx, &pullResult)
		if err != nil {
			_ = setResourceFailed(ctx, "clusters", clusterID)
			return err
		}

		// Create the container.
		var createResult activity.CreateContainerResult
		err = workflow.ExecuteActivity(ctx, "CreateContainer", activity.CreateContainerParams{
			Host:    host,
			Name:    infra.name,
			Image:   infra.image,
			Network: clusterConfig.DockerNetwork,
		}).Get(ctx, &createResult)
		if err != nil {
			_ = setResourceFailed(ctx, "clusters", clusterID)
			return err
		}

		// Wait for healthy.
		err = workflow.ExecuteActivity(ctx, "WaitForHealthy", activity.WaitForHealthyParams{
			Host:        host,
			ContainerID: createResult.ContainerID,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "clusters", clusterID)
			return err
		}

		// Record the infrastructure service.
		now := workflow.Now(ctx)
		err = workflow.ExecuteActivity(ctx, "CreateInfrastructureService", &model.InfrastructureService{
			ID:            platform.NewID(),
			ClusterID:     clusterID,
			HostMachineID: host.ID,
			ServiceType:   infra.serviceType,
			ContainerID:   createResult.ContainerID,
			ContainerName: infra.name,
			Image:         infra.image,
			Config:        json.RawMessage(`{}`),
			Status:        model.StatusActive,
			CreatedAt:     now,
			UpdatedAt:     now,
		}).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "clusters", clusterID)
			return err
		}
	}

	// Create shards and nodes from spec.
	for _, shardSpec := range spec.Shards {
		now := workflow.Now(ctx)
		shardID := platform.NewID()
		shard := &model.Shard{
			ID:        shardID,
			ClusterID: clusterID,
			Name:      shardSpec.Name,
			Role:      shardSpec.Role,
			LBBackend: fmt.Sprintf("shard-%s", shardSpec.Name),
			Config:    json.RawMessage(`{}`),
			Status:    model.StatusActive,
			CreatedAt: now,
			UpdatedAt: now,
		}
		err = workflow.ExecuteActivity(ctx, "CreateShard", shard).Get(ctx, nil)
		if err != nil {
			_ = setResourceFailed(ctx, "clusters", clusterID)
			return err
		}

		// Create nodes for this shard and provision them as child workflows.
		for i := 0; i < shardSpec.NodeCount; i++ {
			nodeID := platform.NewID()
			node := &model.Node{
				ID:        nodeID,
				ClusterID: clusterID,
				ShardID:   &shardID,
				Hostname:  fmt.Sprintf("%s-node-%d", shardSpec.Name, i),
				Roles:     []string{shardSpec.Role},
				Status:    model.StatusPending,
				CreatedAt: now,
				UpdatedAt: now,
			}
			err = workflow.ExecuteActivity(ctx, "CreateNode", node).Get(ctx, nil)
			if err != nil {
				_ = setResourceFailed(ctx, "clusters", clusterID)
				return err
			}

			// Start child ProvisionNodeWorkflow.
			childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
				WorkflowID: fmt.Sprintf("node-provision-%s", nodeID),
			})
			err = workflow.ExecuteChildWorkflow(childCtx, ProvisionNodeWorkflow, ProvisionNodeParams{
				NodeID:        nodeID,
				DockerNetwork: clusterConfig.DockerNetwork,
			}).Get(ctx, nil)
			if err != nil {
				_ = setResourceFailed(ctx, "clusters", clusterID)
				return err
			}
		}
	}

	// Configure HAProxy backends.
	err = workflow.ExecuteActivity(ctx, "ConfigureHAProxyBackends", activity.ConfigureHAProxyBackendsParams{
		ClusterID: clusterID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "clusters", clusterID)
		return err
	}

	// Run smoke test.
	err = workflow.ExecuteActivity(ctx, "RunClusterSmokeTest", activity.RunClusterSmokeTestParams{
		ClusterID: clusterID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "clusters", clusterID)
		return err
	}

	// Set cluster status to active.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "clusters",
		ID:     clusterID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
}

// DecommissionClusterWorkflow tears down all cluster infrastructure.
func DecommissionClusterWorkflow(ctx workflow.Context, clusterID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set cluster status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "clusters",
		ID:     clusterID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// List all nodes in the cluster and decommission them.
	var cluster model.Cluster
	err = workflow.ExecuteActivity(ctx, "GetClusterByID", clusterID).Get(ctx, &cluster)
	if err != nil {
		_ = setResourceFailed(ctx, "clusters", clusterID)
		return err
	}

	// Set cluster status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "clusters",
		ID:     clusterID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}
