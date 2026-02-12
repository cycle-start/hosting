package workflow

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// ---------- ProvisionClusterWorkflow ----------

type ProvisionClusterWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *ProvisionClusterWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
	s.env.RegisterWorkflowWithOptions(ProvisionNodeWorkflow, workflow.RegisterOptions{
		Name: "ProvisionNodeWorkflow",
	})
}

func (s *ProvisionClusterWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *ProvisionClusterWorkflowTestSuite) TestSuccess() {
	clusterID := "test-cluster-1"
	hostID := "test-host-1"

	spec := model.ClusterSpec{
		Shards: []model.ClusterShardSpec{
			{Name: "web-1", Role: "web", NodeCount: 1},
		},
		Infrastructure: model.ClusterInfrastructureSpec{
			HAProxy:   true,
			ServiceDB: false,
			Valkey:    false,
		},
	}
	specJSON, _ := json.Marshal(spec)

	cluster := model.Cluster{
		ID:   clusterID,
		Name: "test-cluster",
		Spec: specJSON,
	}

	host := model.HostMachine{
		ID:        hostID,
		ClusterID: clusterID,
		Hostname:  "host-1",
		IPAddress: "10.0.0.1",
		Status:    model.StatusActive,
	}

	// Set status to provisioning.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusProvisioning,
	}).Return(nil)

	// Get cluster.
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)

	// List host machines.
	s.env.OnActivity("ListHostMachinesByCluster", mock.Anything, clusterID).Return([]model.HostMachine{host}, nil)

	// Validate host reachable (host goes through Temporal JSON serialization, use mock.Anything).
	s.env.OnActivity("ValidateHostReachable", mock.Anything, mock.Anything).Return(nil)

	// HAProxy infra: SelectHostForInfra -> PullImage -> CreateContainer -> WaitForHealthy -> CreateInfrastructureService
	s.env.OnActivity("SelectHostForInfra", mock.Anything, activity.SelectHostForInfraParams{
		ClusterID:   clusterID,
		ServiceType: model.InfraServiceHAProxy,
	}).Return(&host, nil)
	s.env.OnActivity("PullImage", mock.Anything, mock.Anything).Return(&activity.PullImageResult{Digest: "sha256:abc"}, nil)
	s.env.OnActivity("CreateContainer", mock.Anything, mock.Anything).Return(&activity.CreateContainerResult{ContainerID: "container-123", Ports: map[int]int{9090: 9090}}, nil)
	s.env.OnActivity("WaitForHealthy", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("CreateInfrastructureService", mock.Anything, mock.Anything).Return(nil)

	// Create shard (UUIDs generated at runtime, so use mock.Anything).
	s.env.OnActivity("CreateShard", mock.Anything, mock.Anything).Return(nil)

	// Create node.
	s.env.OnActivity("CreateNode", mock.Anything, mock.Anything).Return(nil)

	// Child ProvisionNodeWorkflow.
	s.env.OnWorkflow(ProvisionNodeWorkflow, mock.Anything, mock.Anything).Return(nil)

	// Configure HAProxy backends.
	s.env.OnActivity("ConfigureHAProxyBackends", mock.Anything, activity.ConfigureHAProxyBackendsParams{
		ClusterID: clusterID,
	}).Return(nil)

	// Run smoke test.
	s.env.OnActivity("RunClusterSmokeTest", mock.Anything, activity.RunClusterSmokeTestParams{
		ClusterID: clusterID,
	}).Return(nil)

	// Set status to active.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(ProvisionClusterWorkflow, clusterID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ProvisionClusterWorkflowTestSuite) TestGetClusterFails_SetsStatusFailed() {
	clusterID := "test-cluster-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(ProvisionClusterWorkflow, clusterID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ProvisionClusterWorkflowTestSuite) TestNoHosts_SetsStatusFailed() {
	clusterID := "test-cluster-3"

	spec := model.ClusterSpec{
		Shards: []model.ClusterShardSpec{
			{Name: "web-1", Role: "web", NodeCount: 1},
		},
	}
	specJSON, _ := json.Marshal(spec)

	cluster := model.Cluster{
		ID:   clusterID,
		Name: "test-cluster",
		Spec: specJSON,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("ListHostMachinesByCluster", mock.Anything, clusterID).Return([]model.HostMachine{}, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(ProvisionClusterWorkflow, clusterID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ProvisionClusterWorkflowTestSuite) TestValidateHostFails_SetsStatusFailed() {
	clusterID := "test-cluster-4"
	hostID := "test-host-4"

	spec := model.ClusterSpec{
		Shards: []model.ClusterShardSpec{
			{Name: "web-1", Role: "web", NodeCount: 1},
		},
	}
	specJSON, _ := json.Marshal(spec)

	cluster := model.Cluster{
		ID:   clusterID,
		Spec: specJSON,
	}
	host := model.HostMachine{
		ID:        hostID,
		ClusterID: clusterID,
		Hostname:  "host-1",
		IPAddress: "10.0.0.1",
		Status:    model.StatusActive,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("ListHostMachinesByCluster", mock.Anything, clusterID).Return([]model.HostMachine{host}, nil)
	// Host goes through Temporal JSON serialization, use mock.Anything.
	s.env.OnActivity("ValidateHostReachable", mock.Anything, mock.Anything).Return(fmt.Errorf("unreachable"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(ProvisionClusterWorkflow, clusterID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DecommissionClusterWorkflow ----------

type DecommissionClusterWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DecommissionClusterWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DecommissionClusterWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DecommissionClusterWorkflowTestSuite) TestSuccess() {
	clusterID := "test-cluster-1"

	cluster := model.Cluster{
		ID:   clusterID,
		Name: "test-cluster",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(&cluster, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusDeleted,
	}).Return(nil)

	s.env.ExecuteWorkflow(DecommissionClusterWorkflow, clusterID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DecommissionClusterWorkflowTestSuite) TestGetClusterFails_SetsStatusFailed() {
	clusterID := "test-cluster-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetClusterByID", mock.Anything, clusterID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "clusters", ID: clusterID, Status: model.StatusFailed,
	}).Return(nil)

	s.env.ExecuteWorkflow(DecommissionClusterWorkflow, clusterID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestProvisionClusterWorkflow(t *testing.T) {
	suite.Run(t, new(ProvisionClusterWorkflowTestSuite))
}

func TestDecommissionClusterWorkflow(t *testing.T) {
	suite.Run(t, new(DecommissionClusterWorkflowTestSuite))
}
