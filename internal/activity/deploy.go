package activity

import (
	"context"
	"fmt"
	"time"

	"github.com/edvin/hosting/internal/deployer"
	"github.com/edvin/hosting/internal/model"
)

// DeployResources holds resource constraints for a container (serializable via Temporal).
type DeployResources struct {
	MemoryMB  int64 `json:"memory_mb"`
	CPUShares int64 `json:"cpu_shares"`
}

// DeployHealthCheck holds health check config (serializable via Temporal).
type DeployHealthCheck struct {
	Test     []string      `json:"test"`
	Interval time.Duration `json:"interval"`
	Timeout  time.Duration `json:"timeout"`
	Retries  int           `json:"retries"`
}

// Deploy contains activities for deploying node containers via Docker.
type Deploy struct {
	deployer deployer.Deployer
}

// NewDeploy creates a new Deploy activity struct.
func NewDeploy(d deployer.Deployer) *Deploy {
	return &Deploy{deployer: d}
}

// PullImageParams holds parameters for PullImage.
type PullImageParams struct {
	Host  model.HostMachine `json:"host"`
	Image string            `json:"image"`
}

// PullImageResult holds the result of PullImage.
type PullImageResult struct {
	Digest string `json:"digest"`
}

// PullImage pulls a container image on a host machine.
func (a *Deploy) PullImage(ctx context.Context, params PullImageParams) (*PullImageResult, error) {
	digest, err := a.deployer.PullImage(ctx, &params.Host, params.Image)
	if err != nil {
		return nil, fmt.Errorf("pull image: %w", err)
	}
	return &PullImageResult{Digest: digest}, nil
}

// CreateContainerParams holds parameters for CreateContainer.
type CreateContainerParams struct {
	Host        model.HostMachine      `json:"host"`
	Name        string                 `json:"name"`
	Image       string                 `json:"image"`
	Env         map[string]string      `json:"env"`
	Volumes     []string               `json:"volumes"`
	Ports       []deployer.PortMapping `json:"ports"`
	Resources   DeployResources        `json:"resources"`
	HealthCheck *DeployHealthCheck     `json:"health_check"`
	Privileged  bool                   `json:"privileged"`
	NetworkMode string                 `json:"network_mode"`
	Network     string                 `json:"network,omitempty"`
}

// CreateContainerResult holds the result of CreateContainer.
type CreateContainerResult struct {
	ContainerID string      `json:"container_id"`
	Ports       map[int]int `json:"ports"` // container port -> actual host port
}

// CreateContainer creates and starts a container on a host machine.
func (a *Deploy) CreateContainer(ctx context.Context, params CreateContainerParams) (*CreateContainerResult, error) {
	opts := deployer.ContainerOpts{
		Name:    params.Name,
		Image:   params.Image,
		Env:     params.Env,
		Volumes: params.Volumes,
		Ports:   params.Ports,
		Resources: deployer.Resources{
			MemoryMB:  params.Resources.MemoryMB,
			CPUShares: params.Resources.CPUShares,
		},
		Privileged:  params.Privileged,
		NetworkMode: params.NetworkMode,
		Network:     params.Network,
	}
	if params.HealthCheck != nil {
		opts.HealthCheck = &deployer.HealthCheck{
			Test:     params.HealthCheck.Test,
			Interval: params.HealthCheck.Interval,
			Timeout:  params.HealthCheck.Timeout,
			Retries:  params.HealthCheck.Retries,
		}
	}
	result, err := a.deployer.CreateContainer(ctx, &params.Host, opts)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	return &CreateContainerResult{
		ContainerID: result.ContainerID,
		Ports:       result.Ports,
	}, nil
}

// StopContainerParams holds parameters for StopContainer.
type StopContainerParams struct {
	Host        model.HostMachine `json:"host"`
	ContainerID string            `json:"container_id"`
}

// StopContainer stops a container on a host machine.
func (a *Deploy) StopContainer(ctx context.Context, params StopContainerParams) error {
	return a.deployer.StopContainer(ctx, &params.Host, params.ContainerID)
}

// RemoveContainerParams holds parameters for RemoveContainer.
type RemoveContainerParams struct {
	Host        model.HostMachine `json:"host"`
	ContainerID string            `json:"container_id"`
}

// RemoveContainer removes a container from a host machine.
func (a *Deploy) RemoveContainer(ctx context.Context, params RemoveContainerParams) error {
	return a.deployer.RemoveContainer(ctx, &params.Host, params.ContainerID)
}

// WaitForHealthyParams holds parameters for WaitForHealthy.
type WaitForHealthyParams struct {
	Host        model.HostMachine `json:"host"`
	ContainerID string            `json:"container_id"`
	NodeID      string            `json:"node_id"`
}

// WaitForHealthy polls container health until it becomes healthy or times out.
func (a *Deploy) WaitForHealthy(ctx context.Context, params WaitForHealthyParams) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 24; i++ { // 2 minutes max
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := a.deployer.InspectContainer(ctx, &params.Host, params.ContainerID)
			if err != nil {
				continue
			}
			if status.Health == "healthy" || (status.Health == "none" && status.Running) {
				return nil
			}
		}
	}
	return fmt.Errorf("container %s did not become healthy within timeout", params.ContainerID)
}
