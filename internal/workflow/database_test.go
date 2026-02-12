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

// ---------- CreateDatabaseWorkflow ----------

type CreateDatabaseWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateDatabaseWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateDatabaseWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateDatabaseWorkflowTestSuite) TestSuccess() {
	databaseID := "test-database-1"
	shardID := "test-shard-1"
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateDatabase", mock.Anything, "mydb").Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateDatabaseWorkflow, databaseID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateDatabaseWorkflowTestSuite) TestGetDatabaseFails_SetsStatusFailed() {
	databaseID := "test-database-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateDatabaseWorkflow, databaseID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateDatabaseWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	databaseID := "test-database-3"
	shardID := "test-shard-3"
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateDatabase", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateDatabaseWorkflow, databaseID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateDatabaseWorkflowTestSuite) TestSetProvisioningFails() {
	databaseID := "test-database-4"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))
	s.env.ExecuteWorkflow(CreateDatabaseWorkflow, databaseID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteDatabaseWorkflow ----------

type DeleteDatabaseWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteDatabaseWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteDatabaseWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteDatabaseWorkflowTestSuite) TestSuccess() {
	databaseID := "test-database-1"
	shardID := "test-shard-1"
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteDatabase", mock.Anything, "mydb").Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteDatabaseWorkflow, databaseID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteDatabaseWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	databaseID := "test-database-2"
	shardID := "test-shard-2"
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteDatabase", mock.Anything, "mydb").Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteDatabaseWorkflow, databaseID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteDatabaseWorkflowTestSuite) TestGetDatabaseFails_SetsStatusFailed() {
	databaseID := "test-database-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteDatabaseWorkflow, databaseID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateDatabaseWorkflow(t *testing.T) {
	suite.Run(t, new(CreateDatabaseWorkflowTestSuite))
}

func TestDeleteDatabaseWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteDatabaseWorkflowTestSuite))
}
