package workflow

import (
	"encoding/json"
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/deployer"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
)

// resolveStorageVolumes returns shared storage volume mounts for a node based
// on the shard role and deployment environment. Only web shards get shared
// storage. In Docker (dev), named volumes are used; in production, CephFS bind
// mounts from cephMountBase are used.
func resolveStorageVolumes(shard model.Shard, dockerNetwork, cephMountBase string) []string {
	if shard.Role != model.ShardRoleWeb {
		return nil
	}
	if dockerNetwork != "" {
		return []string{
			fmt.Sprintf("hosting-%s-storage:/var/www/storage", shard.Name),
			fmt.Sprintf("hosting-%s-homes:/home", shard.Name),
		}
	}
	if cephMountBase != "" {
		return []string{
			fmt.Sprintf("%s/%s/storage:/var/www/storage", cephMountBase, shard.Name),
			fmt.Sprintf("%s/%s/homes:/home", cephMountBase, shard.Name),
		}
	}
	return nil
}

// ProvisionNodeParams holds parameters for the ProvisionNodeWorkflow.
type ProvisionNodeParams struct {
	NodeID        string `json:"node_id"`
	DockerNetwork string `json:"docker_network,omitempty"`
	CephMountBase string `json:"ceph_mount_base,omitempty"`
}

// ProvisionNodeWorkflow deploys a node container on a host machine.
func ProvisionNodeWorkflow(ctx workflow.Context, params ProvisionNodeParams) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	nodeID := params.NodeID

	// Set node status to provisioning.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "nodes",
		ID:     nodeID,
		Status: model.StatusProvisioning,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the node.
	var node model.Node
	err = workflow.ExecuteActivity(ctx, "GetNodeByID", nodeID).Get(ctx, &node)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	if node.ShardID == nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return fmt.Errorf("node %s has no shard assignment", nodeID)
	}

	// Get the shard to determine role.
	var shard model.Shard
	err = workflow.ExecuteActivity(ctx, "GetShardByID", *node.ShardID).Get(ctx, &shard)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Get the profile for this role.
	var profile model.NodeProfile
	err = workflow.ExecuteActivity(ctx, "GetNodeProfileByRole", shard.Role).Get(ctx, &profile)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Select a host machine in the cluster.
	var host model.HostMachine
	err = workflow.ExecuteActivity(ctx, "SelectHostForNode", activity.SelectHostForNodeParams{
		ClusterID: node.ClusterID,
	}).Get(ctx, &host)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Pull image on target host.
	var pullResult activity.PullImageResult
	err = workflow.ExecuteActivity(ctx, "PullImage", activity.PullImageParams{
		Host:  host,
		Image: profile.Image,
	}).Get(ctx, &pullResult)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Build environment: merge profile defaults + node-specific overrides.
	env := make(map[string]string)
	var profileEnv map[string]string
	if err := json.Unmarshal(profile.Env, &profileEnv); err == nil {
		for k, v := range profileEnv {
			env[k] = v
		}
	}
	env["NODE_ID"] = nodeID
	env["SHARD_ID"] = *node.ShardID
	env["SHARD_NAME"] = shard.Name
	env["CLUSTER_ID"] = node.ClusterID
	env["SHARD_ROLE"] = shard.Role

	// Parse ports from profile. When using Docker networking, skip host port bindings.
	var ports []deployer.PortMapping
	if params.DockerNetwork == "" {
		_ = json.Unmarshal(profile.Ports, &ports)
	}

	// Parse volumes from profile.
	var volumes []string
	_ = json.Unmarshal(profile.Volumes, &volumes)

	// Append shared storage volumes for web shards.
	volumes = append(volumes, resolveStorageVolumes(shard, params.DockerNetwork, params.CephMountBase)...)

	// Parse resources from profile.
	var resources struct {
		MemoryMB  int64 `json:"memory_mb"`
		CPUShares int64 `json:"cpu_shares"`
	}
	_ = json.Unmarshal(profile.Resources, &resources)

	containerName := fmt.Sprintf("node-%s", nodeID[:8])

	// Create and start the container.
	var createResult activity.CreateContainerResult
	err = workflow.ExecuteActivity(ctx, "CreateContainer", activity.CreateContainerParams{
		Host:    host,
		Name:    containerName,
		Image:   profile.Image,
		Env:     env,
		Volumes: volumes,
		Ports:   ports,
		Resources: activity.DeployResources{
			MemoryMB:  resources.MemoryMB,
			CPUShares: resources.CPUShares,
		},
		Privileged:  profile.Privileged,
		NetworkMode: profile.NetworkMode,
		Network:     params.DockerNetwork,
	}).Get(ctx, &createResult)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Create deployment record.
	now := workflow.Now(ctx)
	envOverrides, _ := json.Marshal(env)
	err = workflow.ExecuteActivity(ctx, "CreateNodeDeployment", &model.NodeDeployment{
		ID:            platform.NewID(),
		NodeID:        nodeID,
		HostMachineID: host.ID,
		ProfileID:     profile.ID,
		ContainerID:   createResult.ContainerID,
		ContainerName: containerName,
		ImageDigest:   pullResult.Digest,
		EnvOverrides:  envOverrides,
		Status:        model.StatusActive,
		DeployedAt:    &now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Wait for the container to be healthy.
	err = workflow.ExecuteActivity(ctx, "WaitForHealthy", activity.WaitForHealthyParams{
		Host:        host,
		ContainerID: createResult.ContainerID,
		NodeID:      nodeID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Set gRPC address. With Docker networking, use container name; otherwise use host IP + mapped port.
	var grpcAddr string
	if params.DockerNetwork != "" {
		grpcAddr = fmt.Sprintf("%s:9090", containerName)
	} else {
		grpcPort := 9090
		if actualPort, ok := createResult.Ports[9090]; ok {
			grpcPort = actualPort
		}
		grpcAddr = fmt.Sprintf("%s:%d", host.IPAddress, grpcPort)
	}
	err = workflow.ExecuteActivity(ctx, "UpdateNodeGRPCAddress", nodeID, grpcAddr).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Set node status to active.
	err = workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "nodes",
		ID:     nodeID,
		Status: model.StatusActive,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Converge the shard to push existing resources to the new node.
	if node.ShardID != nil {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("converge-shard-%s-node-%s", *node.ShardID, nodeID),
		})
		convergeErr := workflow.ExecuteChildWorkflow(childCtx, ConvergeShardWorkflow, ConvergeShardParams{
			ShardID: *node.ShardID,
		}).Get(ctx, nil)
		if convergeErr != nil {
			workflow.GetLogger(ctx).Warn("shard convergence failed after node provision", "nodeID", nodeID, "error", convergeErr)
		}
	}

	return nil
}

// DecommissionNodeWorkflow stops and removes a node container.
func DecommissionNodeWorkflow(ctx workflow.Context, nodeID string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Set node status to deleting.
	err := workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "nodes",
		ID:     nodeID,
		Status: model.StatusDeleting,
	}).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Get the deployment.
	var deployment model.NodeDeployment
	err = workflow.ExecuteActivity(ctx, "GetNodeDeploymentByNodeID", nodeID).Get(ctx, &deployment)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Get the host machine.
	var host model.HostMachine
	err = workflow.ExecuteActivity(ctx, "GetHostMachineByID", deployment.HostMachineID).Get(ctx, &host)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Stop the container.
	err = workflow.ExecuteActivity(ctx, "StopContainer", activity.StopContainerParams{
		Host:        host,
		ContainerID: deployment.ContainerID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Remove the container.
	err = workflow.ExecuteActivity(ctx, "RemoveContainer", activity.RemoveContainerParams{
		Host:        host,
		ContainerID: deployment.ContainerID,
	}).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Update deployment status.
	deployment.Status = model.StatusDeleted
	err = workflow.ExecuteActivity(ctx, "UpdateNodeDeployment", &deployment).Get(ctx, nil)
	if err != nil {
		_ = setResourceFailed(ctx, "nodes", nodeID)
		return err
	}

	// Set node status to deleted.
	return workflow.ExecuteActivity(ctx, "UpdateResourceStatus", activity.UpdateResourceStatusParams{
		Table:  "nodes",
		ID:     nodeID,
		Status: model.StatusDeleted,
	}).Get(ctx, nil)
}

// RollingUpdateParams holds parameters for the RollingUpdateWorkflow.
type RollingUpdateParams struct {
	ShardID       string `json:"shard_id"`
	NewImage      string `json:"new_image"`
	DockerNetwork string `json:"docker_network,omitempty"`
	CephMountBase string `json:"ceph_mount_base,omitempty"`
}

// RollingUpdateWorkflow updates all nodes in a shard to a new image one at a time.
func RollingUpdateWorkflow(ctx workflow.Context, params RollingUpdateParams) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// List all nodes in the shard.
	var nodes []model.Node
	err := workflow.ExecuteActivity(ctx, "ListNodesByShard", params.ShardID).Get(ctx, &nodes)
	if err != nil {
		return err
	}

	// Get the shard and profile for volume resolution.
	var shard model.Shard
	err = workflow.ExecuteActivity(ctx, "GetShardByID", params.ShardID).Get(ctx, &shard)
	if err != nil {
		return fmt.Errorf("get shard: %w", err)
	}

	var profile model.NodeProfile
	err = workflow.ExecuteActivity(ctx, "GetNodeProfileByRole", shard.Role).Get(ctx, &profile)
	if err != nil {
		return fmt.Errorf("get node profile: %w", err)
	}

	var profileVolumes []string
	_ = json.Unmarshal(profile.Volumes, &profileVolumes)
	volumes := append(profileVolumes, resolveStorageVolumes(shard, params.DockerNetwork, params.CephMountBase)...)

	for _, node := range nodes {
		// Get the deployment for this node.
		var deployment model.NodeDeployment
		err = workflow.ExecuteActivity(ctx, "GetNodeDeploymentByNodeID", node.ID).Get(ctx, &deployment)
		if err != nil {
			return fmt.Errorf("get deployment for node %s: %w", node.ID, err)
		}

		// Get the host machine.
		var host model.HostMachine
		err = workflow.ExecuteActivity(ctx, "GetHostMachineByID", deployment.HostMachineID).Get(ctx, &host)
		if err != nil {
			return fmt.Errorf("get host for node %s: %w", node.ID, err)
		}

		// Pull the new image.
		var pullResult activity.PullImageResult
		err = workflow.ExecuteActivity(ctx, "PullImage", activity.PullImageParams{
			Host:  host,
			Image: params.NewImage,
		}).Get(ctx, &pullResult)
		if err != nil {
			return fmt.Errorf("pull image for node %s: %w", node.ID, err)
		}

		// Stop the old container.
		err = workflow.ExecuteActivity(ctx, "StopContainer", activity.StopContainerParams{
			Host:        host,
			ContainerID: deployment.ContainerID,
		}).Get(ctx, nil)
		if err != nil {
			return fmt.Errorf("stop container for node %s: %w", node.ID, err)
		}

		// Remove the old container.
		err = workflow.ExecuteActivity(ctx, "RemoveContainer", activity.RemoveContainerParams{
			Host:        host,
			ContainerID: deployment.ContainerID,
		}).Get(ctx, nil)
		if err != nil {
			return fmt.Errorf("remove container for node %s: %w", node.ID, err)
		}

		// Parse env overrides from deployment.
		var envOverrides map[string]string
		_ = json.Unmarshal(deployment.EnvOverrides, &envOverrides)

		// Create a new container with the new image.
		var createResult activity.CreateContainerResult
		err = workflow.ExecuteActivity(ctx, "CreateContainer", activity.CreateContainerParams{
			Host:        host,
			Name:        deployment.ContainerName,
			Image:       params.NewImage,
			Env:         envOverrides,
			Volumes:     volumes,
			NetworkMode: "bridge",
		}).Get(ctx, &createResult)
		if err != nil {
			return fmt.Errorf("create container for node %s: %w", node.ID, err)
		}

		// Wait for healthy.
		err = workflow.ExecuteActivity(ctx, "WaitForHealthy", activity.WaitForHealthyParams{
			Host:        host,
			ContainerID: createResult.ContainerID,
			NodeID:      node.ID,
		}).Get(ctx, nil)
		if err != nil {
			return fmt.Errorf("wait for healthy for node %s: %w", node.ID, err)
		}

		// Update deployment record.
		deployment.ContainerID = createResult.ContainerID
		deployment.ImageDigest = pullResult.Digest
		err = workflow.ExecuteActivity(ctx, "UpdateNodeDeployment", &deployment).Get(ctx, nil)
		if err != nil {
			return fmt.Errorf("update deployment for node %s: %w", node.ID, err)
		}
	}

	return nil
}
