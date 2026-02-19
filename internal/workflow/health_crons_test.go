package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// ---------- CheckConvergenceHealthWorkflow ----------

type CheckConvergenceHealthWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CheckConvergenceHealthWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CheckConvergenceHealthWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CheckConvergenceHealthWorkflowTestSuite) TestNoStaleShards() {
	s.env.OnActivity("FindStaleConvergingShards", mock.Anything, mock.Anything).
		Return([]activity.StaleConvergingShard{}, nil)

	// All roles return no shards — nothing to auto-resolve.
	s.env.OnActivity("ListShardsByRole", mock.Anything, mock.Anything).
		Return([]model.Shard{}, nil)

	s.env.ExecuteWorkflow(CheckConvergenceHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckConvergenceHealthWorkflowTestSuite) TestStaleShardCreatesIncident() {
	staleShard := activity.StaleConvergingShard{
		ID:        "shard-web-1",
		ClusterID: "osl-1",
		Name:      "web-1",
		Role:      "web",
	}

	s.env.OnActivity("FindStaleConvergingShards", mock.Anything, mock.Anything).
		Return([]activity.StaleConvergingShard{staleShard}, nil)

	// Should create incident for stale shard.
	s.env.OnActivity("CreateIncident", mock.Anything, mock.MatchedBy(func(p activity.CreateIncidentParams) bool {
		return p.Type == "convergence_stuck" && p.Severity == "warning" &&
			*p.ResourceID == "shard-web-1"
	})).Return(&activity.CreateIncidentResult{ID: "inc-1", Created: true}, nil)

	// ListShardsByRole returns the stale shard (still converging) — should NOT auto-resolve.
	s.env.OnActivity("ListShardsByRole", mock.Anything, model.ShardRoleWeb).
		Return([]model.Shard{{ID: "shard-web-1", Status: model.StatusConverging}}, nil)
	// Other roles return empty.
	s.env.OnActivity("ListShardsByRole", mock.Anything, mock.MatchedBy(func(role string) bool {
		return role != model.ShardRoleWeb
	})).Return([]model.Shard{}, nil)

	s.env.ExecuteWorkflow(CheckConvergenceHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckConvergenceHealthWorkflowTestSuite) TestRecoveredShardAutoResolves() {
	// No stale shards.
	s.env.OnActivity("FindStaleConvergingShards", mock.Anything, mock.Anything).
		Return([]activity.StaleConvergingShard{}, nil)

	// One active shard (not converging) — should auto-resolve.
	s.env.OnActivity("ListShardsByRole", mock.Anything, model.ShardRoleWeb).
		Return([]model.Shard{{ID: "shard-web-1", Status: model.StatusActive}}, nil)
	s.env.OnActivity("ListShardsByRole", mock.Anything, mock.MatchedBy(func(role string) bool {
		return role != model.ShardRoleWeb
	})).Return([]model.Shard{}, nil)

	s.env.OnActivity("AutoResolveIncidents", mock.Anything, mock.MatchedBy(func(p activity.AutoResolveIncidentsParams) bool {
		return p.ResourceType == "shard" && p.ResourceID == "shard-web-1" && p.TypePrefix == "convergence_"
	})).Return(1, nil)

	s.env.ExecuteWorkflow(CheckConvergenceHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckConvergenceHealthWorkflowTestSuite) TestFindStaleShardsFails() {
	s.env.OnActivity("FindStaleConvergingShards", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("db connection lost"))

	s.env.ExecuteWorkflow(CheckConvergenceHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func TestCheckConvergenceHealthWorkflow(t *testing.T) {
	suite.Run(t, new(CheckConvergenceHealthWorkflowTestSuite))
}

// ---------- CheckNodeHealthWorkflow ----------

type CheckNodeHealthWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CheckNodeHealthWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CheckNodeHealthWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CheckNodeHealthWorkflowTestSuite) TestAllNodesHealthy() {
	s.env.OnActivity("FindUnhealthyNodes", mock.Anything, mock.Anything).
		Return([]model.Node{}, nil)

	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return([]model.Node{{ID: "node-1", Hostname: "web-0"}}, nil)

	// Should auto-resolve for the healthy node.
	s.env.OnActivity("AutoResolveIncidents", mock.Anything, mock.MatchedBy(func(p activity.AutoResolveIncidentsParams) bool {
		return p.ResourceType == "node" && p.ResourceID == "node-1" && p.TypePrefix == "node_health_"
	})).Return(0, nil)

	s.env.ExecuteWorkflow(CheckNodeHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckNodeHealthWorkflowTestSuite) TestUnhealthyNodeCreatesIncident() {
	lastHealth := time.Now().Add(-10 * time.Minute)
	unhealthyNode := model.Node{
		ID:           "node-1",
		Hostname:     "web-0",
		LastHealthAt: &lastHealth,
		Status:       model.StatusActive,
	}

	s.env.OnActivity("FindUnhealthyNodes", mock.Anything, mock.Anything).
		Return([]model.Node{unhealthyNode}, nil)

	s.env.OnActivity("CreateIncident", mock.Anything, mock.MatchedBy(func(p activity.CreateIncidentParams) bool {
		return p.Type == "node_health_missing" && p.Severity == "critical" &&
			*p.ResourceID == "node-1"
	})).Return(&activity.CreateIncidentResult{ID: "inc-1", Created: true}, nil)

	// ListActiveNodes returns both healthy and unhealthy — only healthy should auto-resolve.
	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return([]model.Node{
			{ID: "node-1", Hostname: "web-0"},
			{ID: "node-2", Hostname: "web-1"},
		}, nil)

	s.env.OnActivity("AutoResolveIncidents", mock.Anything, mock.MatchedBy(func(p activity.AutoResolveIncidentsParams) bool {
		return p.ResourceID == "node-2"
	})).Return(0, nil)

	s.env.ExecuteWorkflow(CheckNodeHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckNodeHealthWorkflowTestSuite) TestNeverReportedNode() {
	unhealthyNode := model.Node{
		ID:           "node-1",
		Hostname:     "web-0",
		LastHealthAt: nil,
		Status:       model.StatusActive,
	}

	s.env.OnActivity("FindUnhealthyNodes", mock.Anything, mock.Anything).
		Return([]model.Node{unhealthyNode}, nil)

	s.env.OnActivity("CreateIncident", mock.Anything, mock.MatchedBy(func(p activity.CreateIncidentParams) bool {
		return p.Type == "node_health_missing" && p.Detail == "Node web-0 (node-1) has never reported health"
	})).Return(&activity.CreateIncidentResult{ID: "inc-1", Created: true}, nil)

	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return([]model.Node{{ID: "node-1", Hostname: "web-0"}}, nil)

	s.env.ExecuteWorkflow(CheckNodeHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckNodeHealthWorkflowTestSuite) TestFindUnhealthyNodesFails() {
	s.env.OnActivity("FindUnhealthyNodes", mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("db connection lost"))

	s.env.ExecuteWorkflow(CheckNodeHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func TestCheckNodeHealthWorkflow(t *testing.T) {
	suite.Run(t, new(CheckNodeHealthWorkflowTestSuite))
}

// ---------- CheckDiskPressureWorkflow ----------

type CheckDiskPressureWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CheckDiskPressureWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CheckDiskPressureWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CheckDiskPressureWorkflowTestSuite) TestAllDisksHealthy() {
	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return([]model.Node{{ID: "node-1", Hostname: "web-0"}}, nil)

	s.env.OnActivity("GetDiskUsage", mock.Anything).
		Return([]activity.DiskUsage{
			{Path: "/", TotalBytes: 100e9, UsedBytes: 50e9, FreeBytes: 50e9, UsedPct: 50},
		}, nil)

	s.env.OnActivity("AutoResolveIncidents", mock.Anything, mock.MatchedBy(func(p activity.AutoResolveIncidentsParams) bool {
		return p.ResourceType == "node" && p.ResourceID == "node-1" && p.TypePrefix == "disk_pressure"
	})).Return(0, nil)

	s.env.ExecuteWorkflow(CheckDiskPressureWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckDiskPressureWorkflowTestSuite) TestHighDiskUsageWarning() {
	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return([]model.Node{{ID: "node-1", Hostname: "web-0"}}, nil)

	s.env.OnActivity("GetDiskUsage", mock.Anything).
		Return([]activity.DiskUsage{
			{Path: "/", TotalBytes: 100e9, UsedBytes: 92e9, FreeBytes: 8e9, UsedPct: 92},
		}, nil)

	s.env.OnActivity("CreateIncident", mock.Anything, mock.MatchedBy(func(p activity.CreateIncidentParams) bool {
		return p.Type == "disk_pressure" && p.Severity == "warning" &&
			*p.ResourceID == "node-1"
	})).Return(&activity.CreateIncidentResult{ID: "inc-1", Created: true}, nil)

	s.env.ExecuteWorkflow(CheckDiskPressureWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckDiskPressureWorkflowTestSuite) TestCriticalDiskUsage() {
	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return([]model.Node{{ID: "node-1", Hostname: "web-0"}}, nil)

	s.env.OnActivity("GetDiskUsage", mock.Anything).
		Return([]activity.DiskUsage{
			{Path: "/", TotalBytes: 100e9, UsedBytes: 97e9, FreeBytes: 3e9, UsedPct: 97},
		}, nil)

	s.env.OnActivity("CreateIncident", mock.Anything, mock.MatchedBy(func(p activity.CreateIncidentParams) bool {
		return p.Type == "disk_pressure" && p.Severity == "critical"
	})).Return(&activity.CreateIncidentResult{ID: "inc-1", Created: true}, nil)

	s.env.ExecuteWorkflow(CheckDiskPressureWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckDiskPressureWorkflowTestSuite) TestNodeUnreachableSkipped() {
	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return([]model.Node{
			{ID: "node-1", Hostname: "web-0"},
			{ID: "node-2", Hostname: "web-1"},
		}, nil)

	// First node unreachable, second healthy.
	s.env.OnActivity("GetDiskUsage", mock.Anything).Return(nil, fmt.Errorf("activity timeout")).Once()
	s.env.OnActivity("GetDiskUsage", mock.Anything).
		Return([]activity.DiskUsage{
			{Path: "/", TotalBytes: 100e9, UsedBytes: 50e9, FreeBytes: 50e9, UsedPct: 50},
		}, nil).Once()

	s.env.OnActivity("AutoResolveIncidents", mock.Anything, mock.MatchedBy(func(p activity.AutoResolveIncidentsParams) bool {
		return p.ResourceID == "node-2"
	})).Return(0, nil)

	s.env.ExecuteWorkflow(CheckDiskPressureWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckDiskPressureWorkflowTestSuite) TestListActiveNodesFails() {
	s.env.OnActivity("ListActiveNodes", mock.Anything).
		Return(nil, fmt.Errorf("db connection lost"))

	s.env.ExecuteWorkflow(CheckDiskPressureWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func TestCheckDiskPressureWorkflow(t *testing.T) {
	suite.Run(t, new(CheckDiskPressureWorkflowTestSuite))
}
