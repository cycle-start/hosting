package deployer

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/edvin/hosting/internal/model"
)

// DockerDeployer implements Deployer using the Docker API.
type DockerDeployer struct{}

// NewDockerDeployer creates a new DockerDeployer.
func NewDockerDeployer() *DockerDeployer {
	return &DockerDeployer{}
}

func (d *DockerDeployer) clientForHost(host *model.HostMachine) (*client.Client, error) {
	opts := []client.Opt{
		client.WithHost(host.DockerHost),
		client.WithAPIVersionNegotiation(),
	}

	if host.CACertPEM != "" && host.ClientCertPEM != "" && host.ClientKeyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(host.ClientCertPEM), []byte(host.ClientKeyPEM))
		if err != nil {
			return nil, fmt.Errorf("parse client cert: %w", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(host.CACertPEM))

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caCertPool,
		}
		httpClient := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConfig,
			},
		}
		opts = append(opts, client.WithHTTPClient(httpClient))
	}

	return client.NewClientWithOpts(opts...)
}

func (d *DockerDeployer) PullImage(ctx context.Context, host *model.HostMachine, img string) (string, error) {
	cli, err := d.clientForHost(host)
	if err != nil {
		return "", fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	reader, err := cli.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return "", fmt.Errorf("pull image %s: %w", img, err)
	}
	defer reader.Close()
	// Drain the pull output.
	_, _ = io.Copy(io.Discard, reader)

	inspect, _, err := cli.ImageInspectWithRaw(ctx, img)
	if err != nil {
		return "", fmt.Errorf("inspect image %s: %w", img, err)
	}

	digest := ""
	if len(inspect.RepoDigests) > 0 {
		digest = inspect.RepoDigests[0]
	}
	return digest, nil
}

func (d *DockerDeployer) CreateContainer(ctx context.Context, host *model.HostMachine, opts ContainerOpts) (*CreateResult, error) {
	cli, err := d.clientForHost(host)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	env := make([]string, 0, len(opts.Env))
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}

	exposedPorts := nat.PortSet{}
	portBindings := nat.PortMap{}
	for _, pm := range opts.Ports {
		cp := nat.Port(strconv.Itoa(pm.Container) + "/tcp")
		exposedPorts[cp] = struct{}{}
		hostPort := strconv.Itoa(pm.Host)
		if pm.Host == 0 {
			hostPort = "" // let Docker pick an ephemeral port
		}
		portBindings[cp] = []nat.PortBinding{
			{HostPort: hostPort},
		}
	}

	config := &container.Config{
		Image:        opts.Image,
		Env:          env,
		ExposedPorts: exposedPorts,
	}

	if opts.HealthCheck != nil {
		config.Healthcheck = &container.HealthConfig{
			Test:     opts.HealthCheck.Test,
			Interval: opts.HealthCheck.Interval,
			Timeout:  opts.HealthCheck.Timeout,
			Retries:  opts.HealthCheck.Retries,
		}
	}

	hostConfig := &container.HostConfig{
		PortBindings: portBindings,
		Binds:        opts.Volumes,
		Privileged:   opts.Privileged,
		NetworkMode:  container.NetworkMode(opts.NetworkMode),
		Resources: container.Resources{
			Memory:    opts.Resources.MemoryMB * 1024 * 1024,
			CPUShares: opts.Resources.CPUShares,
		},
	}

	var networkConfig *network.NetworkingConfig
	if opts.Network != "" {
		networkConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				opts.Network: {},
			},
		}
	}

	resp, err := cli.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, opts.Name)
	if err != nil {
		return nil, fmt.Errorf("create container %s: %w", opts.Name, err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start container %s: %w", opts.Name, err)
	}

	// Inspect to get actual port mappings (needed for ephemeral ports).
	result := &CreateResult{
		ContainerID: resp.ID,
		Ports:       make(map[int]int),
	}
	info, err := cli.ContainerInspect(ctx, resp.ID)
	if err == nil {
		for containerPort, bindings := range info.NetworkSettings.Ports {
			if len(bindings) > 0 && bindings[0].HostPort != "" {
				cp := containerPort.Int()
				hp, _ := strconv.Atoi(bindings[0].HostPort)
				result.Ports[cp] = hp
			}
		}
	}

	return result, nil
}

func (d *DockerDeployer) StartContainer(ctx context.Context, host *model.HostMachine, containerID string) error {
	cli, err := d.clientForHost(host)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	return cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (d *DockerDeployer) StopContainer(ctx context.Context, host *model.HostMachine, containerID string) error {
	cli, err := d.clientForHost(host)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	return cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

func (d *DockerDeployer) RemoveContainer(ctx context.Context, host *model.HostMachine, containerID string) error {
	cli, err := d.clientForHost(host)
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	return cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

func (d *DockerDeployer) InspectContainer(ctx context.Context, host *model.HostMachine, containerID string) (*ContainerStatus, error) {
	cli, err := d.clientForHost(host)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	info, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("inspect container %s: %w", containerID, err)
	}

	health := "none"
	if info.State.Health != nil {
		health = info.State.Health.Status
	}

	return &ContainerStatus{
		ID:      info.ID,
		Name:    info.Name,
		State:   info.State.Status,
		Health:  health,
		Running: info.State.Running,
	}, nil
}

func (d *DockerDeployer) ExecInContainer(ctx context.Context, host *model.HostMachine, containerNameOrID string, cmd []string) (*ExecResult, error) {
	cli, err := d.clientForHost(host)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}
	defer cli.Close()

	execCfg := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}

	execID, err := cli.ContainerExecCreate(ctx, containerNameOrID, execCfg)
	if err != nil {
		return nil, fmt.Errorf("exec create in %s: %w", containerNameOrID, err)
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return nil, fmt.Errorf("exec attach in %s: %w", containerNameOrID, err)
	}
	defer resp.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, resp.Reader); err != nil {
		return nil, fmt.Errorf("exec read output in %s: %w", containerNameOrID, err)
	}

	inspectResp, err := cli.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		return nil, fmt.Errorf("exec inspect in %s: %w", containerNameOrID, err)
	}

	return &ExecResult{
		ExitCode: inspectResp.ExitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}
