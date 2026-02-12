package activity

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/edvin/hosting/internal/deployer"
	"github.com/edvin/hosting/internal/model"
)

// --- Mock deployer for LB tests ---

type mockLBDeployer struct {
	mock.Mock
}

func (m *mockLBDeployer) ExecInContainer(ctx context.Context, host *model.HostMachine, containerNameOrID string, cmd []string) (*deployer.ExecResult, error) {
	args := m.Called(ctx, host, containerNameOrID, cmd)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*deployer.ExecResult), args.Error(1)
}

func (m *mockLBDeployer) PullImage(ctx context.Context, host *model.HostMachine, image string) (string, error) {
	args := m.Called(ctx, host, image)
	return args.String(0), args.Error(1)
}

func (m *mockLBDeployer) CreateContainer(ctx context.Context, host *model.HostMachine, opts deployer.ContainerOpts) (*deployer.CreateResult, error) {
	args := m.Called(ctx, host, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*deployer.CreateResult), args.Error(1)
}

func (m *mockLBDeployer) StartContainer(ctx context.Context, host *model.HostMachine, containerID string) error {
	return m.Called(ctx, host, containerID).Error(0)
}

func (m *mockLBDeployer) StopContainer(ctx context.Context, host *model.HostMachine, containerID string) error {
	return m.Called(ctx, host, containerID).Error(0)
}

func (m *mockLBDeployer) RemoveContainer(ctx context.Context, host *model.HostMachine, containerID string) error {
	return m.Called(ctx, host, containerID).Error(0)
}

func (m *mockLBDeployer) InspectContainer(ctx context.Context, host *model.HostMachine, containerID string) (*deployer.ContainerStatus, error) {
	args := m.Called(ctx, host, containerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*deployer.ContainerStatus), args.Error(1)
}

// --- mock row for LB resolve queries ---

type lbMockRow struct {
	scanFn func(dest ...any) error
}

func (r *lbMockRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

// setupResolveHAProxy sets up the mockDB to return a cluster and host machine
// for resolveHAProxy calls.
func setupResolveHAProxy(db *mockDB, clusterConfig json.RawMessage) {
	now := time.Now()
	// First QueryRow: cluster lookup
	db.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return len(sql) > 20 && sql[0:6] == "SELECT" && containsSubstring(sql, "clusters")
	}), mock.Anything).Return(&lbMockRow{scanFn: func(dest ...any) error {
		*(dest[0].(*string)) = "cluster-1"
		*(dest[1].(*string)) = "region-1"
		*(dest[2].(*string)) = "test-cluster"
		*(dest[3].(*json.RawMessage)) = clusterConfig
		*(dest[4].(*string)) = model.StatusActive
		*(dest[5].(*json.RawMessage)) = json.RawMessage(`{}`)
		*(dest[6].(*time.Time)) = now
		*(dest[7].(*time.Time)) = now
		return nil
	}}).Once()

	// Second QueryRow: host machine lookup
	db.On("QueryRow", mock.Anything, mock.MatchedBy(func(sql string) bool {
		return len(sql) > 20 && sql[0:6] == "SELECT" && containsSubstring(sql, "host_machines")
	}), mock.Anything).Return(&lbMockRow{scanFn: func(dest ...any) error {
		*(dest[0].(*string)) = "host-1"
		*(dest[1].(*string)) = "cluster-1"
		*(dest[2].(*string)) = "localhost"
		*(dest[3].(*string)) = "127.0.0.1"
		*(dest[4].(*string)) = "unix:///var/run/docker.sock"
		*(dest[5].(*string)) = ""
		*(dest[6].(*string)) = ""
		*(dest[7].(*string)) = ""
		*(dest[8].(*json.RawMessage)) = json.RawMessage(`{}`)
		*(dest[9].(*[]string)) = []string{"web"}
		*(dest[10].(*string)) = model.StatusActive
		*(dest[11].(*time.Time)) = now
		*(dest[12].(*time.Time)) = now
		return nil
	}}).Once()
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLB_SetLBMapEntry(t *testing.T) {
	db := &mockDB{}
	dep := &mockLBDeployer{}
	lb := NewLB(dep, db)
	ctx := context.Background()

	setupResolveHAProxy(db, json.RawMessage(`{"haproxy_container":"test-haproxy"}`))
	dep.On("ExecInContainer", ctx, mock.Anything, "test-haproxy", mock.Anything).
		Return(&deployer.ExecResult{ExitCode: 0, Stdout: "\n"}, nil)

	err := lb.SetLBMapEntry(ctx, SetLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
		LBBackend: "shard-web-1",
	})
	require.NoError(t, err)
	dep.AssertExpectations(t)
}

func TestLB_SetLBMapEntry_NonZeroExitCode(t *testing.T) {
	db := &mockDB{}
	dep := &mockLBDeployer{}
	lb := NewLB(dep, db)
	ctx := context.Background()

	setupResolveHAProxy(db, json.RawMessage(`{}`))
	dep.On("ExecInContainer", ctx, mock.Anything, defaultHAProxyContainer, mock.Anything).
		Return(&deployer.ExecResult{ExitCode: 1, Stderr: "some error"}, nil)

	err := lb.SetLBMapEntry(ctx, SetLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
		LBBackend: "shard-web-1",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "some error")
}

func TestLB_DeleteLBMapEntry(t *testing.T) {
	db := &mockDB{}
	dep := &mockLBDeployer{}
	lb := NewLB(dep, db)
	ctx := context.Background()

	setupResolveHAProxy(db, json.RawMessage(`{}`))
	dep.On("ExecInContainer", ctx, mock.Anything, defaultHAProxyContainer, mock.Anything).
		Return(&deployer.ExecResult{ExitCode: 0, Stdout: "\n"}, nil)

	err := lb.DeleteLBMapEntry(ctx, DeleteLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
	})
	require.NoError(t, err)
}

func TestLB_DeleteLBMapEntry_NotFoundIgnored(t *testing.T) {
	db := &mockDB{}
	dep := &mockLBDeployer{}
	lb := NewLB(dep, db)
	ctx := context.Background()

	setupResolveHAProxy(db, json.RawMessage(`{}`))
	dep.On("ExecInContainer", ctx, mock.Anything, defaultHAProxyContainer, mock.Anything).
		Return(&deployer.ExecResult{ExitCode: 1, Stdout: "entry not found\n"}, nil)

	err := lb.DeleteLBMapEntry(ctx, DeleteLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
	})
	require.NoError(t, err, "not found errors should be ignored")
}

func TestLB_DeleteLBMapEntry_RealError(t *testing.T) {
	db := &mockDB{}
	dep := &mockLBDeployer{}
	lb := NewLB(dep, db)
	ctx := context.Background()

	setupResolveHAProxy(db, json.RawMessage(`{}`))
	dep.On("ExecInContainer", ctx, mock.Anything, defaultHAProxyContainer, mock.Anything).
		Return(nil, fmt.Errorf("connection refused"))

	err := lb.DeleteLBMapEntry(ctx, DeleteLBMapEntryParams{
		ClusterID: "cluster-1",
		FQDN:      "example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestLB_ResolveHAProxy_DefaultContainer(t *testing.T) {
	db := &mockDB{}
	dep := &mockLBDeployer{}
	lb := NewLB(dep, db)
	ctx := context.Background()

	setupResolveHAProxy(db, json.RawMessage(`{}`))

	_, containerName, err := lb.resolveHAProxy(ctx, "cluster-1")
	require.NoError(t, err)
	assert.Equal(t, defaultHAProxyContainer, containerName)
}

func TestLB_ResolveHAProxy_CustomContainer(t *testing.T) {
	db := &mockDB{}
	dep := &mockLBDeployer{}
	lb := NewLB(dep, db)
	ctx := context.Background()

	setupResolveHAProxy(db, json.RawMessage(`{"haproxy_container":"my-haproxy"}`))

	_, containerName, err := lb.resolveHAProxy(ctx, "cluster-1")
	require.NoError(t, err)
	assert.Equal(t, "my-haproxy", containerName)
}
