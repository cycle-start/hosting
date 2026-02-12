package workflow

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// ---------- CreateValkeyInstanceWorkflow ----------

type CreateValkeyInstanceWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateValkeyInstanceWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateValkeyInstanceWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateValkeyInstanceWorkflowTestSuite) TestSuccess() {
	instanceID := "test-valkey-1"
	shardID := "test-shard-1"
	instance := model.ValkeyInstance{
		ID:          instanceID,
		Name:        "myvalkey",
		ShardID:     &shardID,
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateValkeyInstance", mock.Anything, activity.CreateValkeyInstanceParams{
		Name:        "myvalkey",
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateValkeyInstanceWorkflowTestSuite) TestGetInstanceFails_SetsStatusFailed() {
	instanceID := "test-valkey-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateValkeyInstanceWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	instanceID := "test-valkey-3"
	shardID := "test-shard-3"
	instance := model.ValkeyInstance{
		ID:          instanceID,
		Name:        "myvalkey",
		ShardID:     &shardID,
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateValkeyInstance", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateValkeyInstanceWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	instanceID := "test-valkey-no-shard"
	instance := model.ValkeyInstance{
		ID:          instanceID,
		Name:        "myvalkey",
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateValkeyInstanceWorkflowTestSuite) TestSetProvisioningFails() {
	instanceID := "test-valkey-4"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))
	s.env.ExecuteWorkflow(CreateValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteValkeyInstanceWorkflow ----------

type DeleteValkeyInstanceWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteValkeyInstanceWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteValkeyInstanceWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteValkeyInstanceWorkflowTestSuite) TestSuccess() {
	instanceID := "test-valkey-1"
	shardID := "test-shard-1"
	instance := model.ValkeyInstance{
		ID:      instanceID,
		Name:    "myvalkey",
		Port:    6379,
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteValkeyInstance", mock.Anything, activity.DeleteValkeyInstanceParams{
		Name: "myvalkey",
		Port: 6379,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteValkeyInstanceWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	instanceID := "test-valkey-2"
	shardID := "test-shard-2"
	instance := model.ValkeyInstance{
		ID:      instanceID,
		Name:    "myvalkey",
		Port:    6379,
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteValkeyInstance", mock.Anything, activity.DeleteValkeyInstanceParams{
		Name: "myvalkey",
		Port: 6379,
	}).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteValkeyInstanceWorkflowTestSuite) TestGetInstanceFails_SetsStatusFailed() {
	instanceID := "test-valkey-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteValkeyInstanceWorkflow, instanceID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateValkeyInstanceWorkflow(t *testing.T) {
	suite.Run(t, new(CreateValkeyInstanceWorkflowTestSuite))
}

func TestDeleteValkeyInstanceWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteValkeyInstanceWorkflowTestSuite))
}
