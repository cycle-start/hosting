package workflow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/agent"
	"github.com/edvin/hosting/internal/model"
)

// ---------- CheckReplicationHealthWorkflow ----------

type CheckReplicationHealthWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CheckReplicationHealthWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CheckReplicationHealthWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CheckReplicationHealthWorkflowTestSuite) TestHealthy() {
	shardID := "shard-db-1"
	shard := model.Shard{
		ID:     shardID,
		Role:   model.ShardRoleDatabase,
		Status: model.StatusActive,
	}
	nodes := []model.Node{
		{ID: "primary-1"},
		{ID: "replica-1"},
	}

	s.env.OnActivity("ListShardsByRole", mock.Anything, model.ShardRoleDatabase).Return([]model.Shard{shard}, nil)
	// dbShardPrimary calls GetShardByID + ListNodesByShard.
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)

	// Replica is healthy - IO and SQL both running.
	s.env.OnActivity("GetReplicationStatus", mock.Anything).Return(&agent.ReplicationStatus{
		IORunning:  true,
		SQLRunning: true,
	}, nil)

	// Healthy shard that was already active — no auto-resolve or status change expected.

	s.env.ExecuteWorkflow(CheckReplicationHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckReplicationHealthWorkflowTestSuite) TestBrokenReplication() {
	shardID := "shard-db-2"
	shard := model.Shard{
		ID:     shardID,
		Role:   model.ShardRoleDatabase,
		Status: model.StatusActive,
	}
	nodes := []model.Node{
		{ID: "primary-1"},
		{ID: "replica-1"},
	}

	s.env.OnActivity("ListShardsByRole", mock.Anything, model.ShardRoleDatabase).Return([]model.Shard{shard}, nil)
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)

	// Replica has IO not running.
	s.env.OnActivity("GetReplicationStatus", mock.Anything).Return(&agent.ReplicationStatus{
		IORunning:  false,
		SQLRunning: true,
		LastError:  "connection refused",
	}, nil)

	// Should set shard to degraded.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.MatchedBy(func(params activity.UpdateResourceStatusParams) bool {
		return params.Table == "shards" &&
			params.ID == shardID &&
			params.Status == "degraded" &&
			params.StatusMessage != nil
	})).Return(nil)

	// Should create an incident.
	s.env.OnActivity("CreateIncident", mock.Anything, mock.MatchedBy(func(params activity.CreateIncidentParams) bool {
		return params.Type == "replication_broken" && params.Severity == "critical"
	})).Return(&activity.CreateIncidentResult{ID: "inc-test", Created: true}, nil)

	s.env.ExecuteWorkflow(CheckReplicationHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckReplicationHealthWorkflowTestSuite) TestDegradedRestoredToActive() {
	shardID := "shard-db-3"
	shard := model.Shard{
		ID:     shardID,
		Role:   model.ShardRoleDatabase,
		Status: "degraded",
	}
	nodes := []model.Node{
		{ID: "primary-1"},
		{ID: "replica-1"},
	}

	s.env.OnActivity("ListShardsByRole", mock.Anything, model.ShardRoleDatabase).Return([]model.Shard{shard}, nil)
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)

	// Replica is now healthy.
	s.env.OnActivity("GetReplicationStatus", mock.Anything).Return(&agent.ReplicationStatus{
		IORunning:  true,
		SQLRunning: true,
	}, nil)

	// Should restore shard to active.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.MatchedBy(func(params activity.UpdateResourceStatusParams) bool {
		return params.Table == "shards" &&
			params.ID == shardID &&
			params.Status == model.StatusActive
	})).Return(nil)

	// Should auto-resolve replication incidents.
	s.env.OnActivity("AutoResolveIncidents", mock.Anything, mock.MatchedBy(func(params activity.AutoResolveIncidentsParams) bool {
		return params.ResourceType == "shard" && params.ResourceID == shardID && params.TypePrefix == "replication_"
	})).Return(0, nil)

	s.env.ExecuteWorkflow(CheckReplicationHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckReplicationHealthWorkflowTestSuite) TestHighReplicationLag() {
	shardID := "shard-db-4"
	shard := model.Shard{
		ID:     shardID,
		Role:   model.ShardRoleDatabase,
		Status: model.StatusActive,
	}
	nodes := []model.Node{
		{ID: "primary-1"},
		{ID: "replica-1"},
	}
	lag := 600

	s.env.OnActivity("ListShardsByRole", mock.Anything, model.ShardRoleDatabase).Return([]model.Shard{shard}, nil)
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)

	// Replica running but with high lag.
	s.env.OnActivity("GetReplicationStatus", mock.Anything).Return(&agent.ReplicationStatus{
		IORunning:     true,
		SQLRunning:    true,
		SecondsBehind: &lag,
	}, nil)

	// Should set shard to degraded.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, mock.MatchedBy(func(params activity.UpdateResourceStatusParams) bool {
		return params.Table == "shards" &&
			params.ID == shardID &&
			params.Status == "degraded" &&
			params.StatusMessage != nil
	})).Return(nil)

	// Should create a warning incident.
	s.env.OnActivity("CreateIncident", mock.Anything, mock.MatchedBy(func(params activity.CreateIncidentParams) bool {
		return params.Type == "replication_lag" && params.Severity == "warning"
	})).Return(&activity.CreateIncidentResult{ID: "inc-test", Created: true}, nil)

	s.env.ExecuteWorkflow(CheckReplicationHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CheckReplicationHealthWorkflowTestSuite) TestListShardsFails() {
	s.env.OnActivity("ListShardsByRole", mock.Anything, model.ShardRoleDatabase).Return(nil, fmt.Errorf("db connection lost"))

	s.env.ExecuteWorkflow(CheckReplicationHealthWorkflow)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run ----------

func TestCheckReplicationHealthWorkflow(t *testing.T) {
	suite.Run(t, new(CheckReplicationHealthWorkflowTestSuite))
}
