package deployer

import (
	"context"
	"time"

	"github.com/edvin/hosting/internal/model"
)

// PortMapping describes a host-to-container port binding.
// Host 0 means "let the runtime pick an ephemeral port".
type PortMapping struct {
	Host      int `json:"host"`
	Container int `json:"container"`
}

// ContainerOpts holds the options for creating a container.
type ContainerOpts struct {
	Name        string
	Image       string
	Env         map[string]string
	Volumes     []string
	Ports       []PortMapping
	Resources   Resources
	HealthCheck *HealthCheck
	Privileged  bool
	NetworkMode string
	Network     string // Docker network to attach to (e.g. "hosting_default")
}

// CreateResult holds the result of creating a container.
type CreateResult struct {
	ContainerID string
	Ports       map[int]int // container port -> actual host port
}

// Resources holds resource constraints for a container.
type Resources struct {
	MemoryMB  int64
	CPUShares int64
}

// HealthCheck holds health check configuration.
type HealthCheck struct {
	Test     []string
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

// ContainerStatus holds the status of a container.
type ContainerStatus struct {
	ID      string
	Name    string
	State   string // running, exited, created, etc.
	Health  string // healthy, unhealthy, starting, none
	Running bool
}

// ExecResult holds the result of executing a command in a container.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// Deployer defines the interface for container deployment operations.
type Deployer interface {
	PullImage(ctx context.Context, host *model.HostMachine, image string) (digest string, err error)
	CreateContainer(ctx context.Context, host *model.HostMachine, opts ContainerOpts) (*CreateResult, error)
	StartContainer(ctx context.Context, host *model.HostMachine, containerID string) error
	StopContainer(ctx context.Context, host *model.HostMachine, containerID string) error
	RemoveContainer(ctx context.Context, host *model.HostMachine, containerID string) error
	InspectContainer(ctx context.Context, host *model.HostMachine, containerID string) (*ContainerStatus, error)
	ExecInContainer(ctx context.Context, host *model.HostMachine, containerNameOrID string, cmd []string) (*ExecResult, error)
}
