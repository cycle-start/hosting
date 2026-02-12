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

// ---------- CreateValkeyUserWorkflow ----------

type CreateValkeyUserWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateValkeyUserWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateValkeyUserWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateValkeyUserWorkflowTestSuite) TestSuccess() {
	userID := "test-vuser-1"
	instanceID := "test-valkey-1"
	shardID := "test-shard-1"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "secret123",
		Privileges:       []string{"allcommands"},
		KeyPattern:       "*",
	}
	instance := model.ValkeyInstance{
		ID:      instanceID,
		Name:    "myvalkey",
		ShardID: &shardID,
		Port:    6379,
	}
	nodes := []model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateValkeyUserOnNode", mock.Anything, activity.CreateValkeyUserOnNodeParams{
		NodeAddress: "node1:9090",
		User: activity.CreateValkeyUserParams{
			InstanceName: "myvalkey",
			Port:         6379,
			Username:     "appuser",
			Password:     "secret123",
			Privileges:   []string{"allcommands"},
			KeyPattern:   "*",
		},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateValkeyUserWorkflowTestSuite) TestGetUserFails_SetsStatusFailed() {
	userID := "test-vuser-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateValkeyUserWorkflowTestSuite) TestGetInstanceFails_SetsStatusFailed() {
	userID := "test-vuser-3"
	instanceID := "test-valkey-3"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "secret123",
		Privileges:       []string{"allcommands"},
		KeyPattern:       "*",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateValkeyUserWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	userID := "test-vuser-4"
	instanceID := "test-valkey-4"
	shardID := "test-shard-4"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "secret123",
		Privileges:       []string{"allcommands"},
		KeyPattern:       "*",
	}
	instance := model.ValkeyInstance{
		ID:      instanceID,
		Name:    "myvalkey",
		ShardID: &shardID,
		Port:    6379,
	}
	nodes := []model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateValkeyUserOnNode", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateValkeyUserWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	userID := "test-vuser-no-shard"
	instanceID := "test-valkey-no-shard"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "secret123",
		Privileges:       []string{"allcommands"},
		KeyPattern:       "*",
	}
	instance := model.ValkeyInstance{
		ID:   instanceID,
		Name: "myvalkey",
		Port: 6379,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateValkeyUserWorkflowTestSuite) TestSetProvisioningFails() {
	userID := "test-vuser-5"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))
	s.env.ExecuteWorkflow(CreateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UpdateValkeyUserWorkflow ----------

type UpdateValkeyUserWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateValkeyUserWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UpdateValkeyUserWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateValkeyUserWorkflowTestSuite) TestSuccess() {
	userID := "test-vuser-1"
	instanceID := "test-valkey-1"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "newsecret456",
		Privileges:       []string{"allcommands"},
		KeyPattern:       "app:*",
	}
	instance := model.ValkeyInstance{
		ID:   instanceID,
		Name: "myvalkey",
		Port: 6379,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("UpdateValkeyUser", mock.Anything, activity.UpdateValkeyUserParams{
		InstanceName: "myvalkey",
		Port:         6379,
		Username:     "appuser",
		Password:     "newsecret456",
		Privileges:   []string{"allcommands"},
		KeyPattern:   "app:*",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateValkeyUserWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	userID := "test-vuser-2"
	instanceID := "test-valkey-2"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "newsecret456",
		Privileges:       []string{"allcommands"},
		KeyPattern:       "*",
	}
	instance := model.ValkeyInstance{
		ID:   instanceID,
		Name: "myvalkey",
		Port: 6379,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("UpdateValkeyUser", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UpdateValkeyUserWorkflowTestSuite) TestGetUserFails_SetsStatusFailed() {
	userID := "test-vuser-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteValkeyUserWorkflow ----------

type DeleteValkeyUserWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteValkeyUserWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteValkeyUserWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteValkeyUserWorkflowTestSuite) TestSuccess() {
	userID := "test-vuser-1"
	instanceID := "test-valkey-1"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "secret123",
	}
	instance := model.ValkeyInstance{
		ID:   instanceID,
		Name: "myvalkey",
		Port: 6379,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("DeleteValkeyUser", mock.Anything, activity.DeleteValkeyUserParams{
		InstanceName: "myvalkey",
		Port:         6379,
		Username:     "appuser",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteValkeyUserWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	userID := "test-vuser-2"
	instanceID := "test-valkey-2"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
		Password:         "secret123",
	}
	instance := model.ValkeyInstance{
		ID:   instanceID,
		Name: "myvalkey",
		Port: 6379,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("DeleteValkeyUser", mock.Anything, activity.DeleteValkeyUserParams{
		InstanceName: "myvalkey",
		Port:         6379,
		Username:     "appuser",
	}).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteValkeyUserWorkflowTestSuite) TestGetUserFails_SetsStatusFailed() {
	userID := "test-vuser-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteValkeyUserWorkflowTestSuite) TestGetInstanceFails_SetsStatusFailed() {
	userID := "test-vuser-4"
	instanceID := "test-valkey-4"

	vUser := model.ValkeyUser{
		ID:               userID,
		ValkeyInstanceID: instanceID,
		Username:         "appuser",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetValkeyUserByID", mock.Anything, userID).Return(&vUser, nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_users", ID: userID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteValkeyUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateValkeyUserWorkflow(t *testing.T) {
	suite.Run(t, new(CreateValkeyUserWorkflowTestSuite))
}

func TestUpdateValkeyUserWorkflow(t *testing.T) {
	suite.Run(t, new(UpdateValkeyUserWorkflowTestSuite))
}

func TestDeleteValkeyUserWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteValkeyUserWorkflowTestSuite))
}
