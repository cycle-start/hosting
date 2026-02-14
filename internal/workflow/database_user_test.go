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

// ---------- CreateDatabaseUserWorkflow ----------

type CreateDatabaseUserWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateDatabaseUserWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateDatabaseUserWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateDatabaseUserWorkflowTestSuite) TestSuccess() {
	userID := "test-dbuser-1"
	databaseID := "test-database-1"
	shardID := "test-shard-1"

	dbUser := model.DatabaseUser{
		ID:         userID,
		DatabaseID: databaseID,
		Username:   "appuser",
		Password:   "secret123",
		Privileges: []string{"SELECT", "INSERT", "UPDATE"},
	}
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(&activity.DatabaseUserContext{
		User:     dbUser,
		Database: database,
		Nodes:    nodes,
	}, nil)
	s.env.OnActivity("CreateDatabaseUser", mock.Anything, activity.CreateDatabaseUserParams{
		DatabaseName: "mydb",
		Username:     "appuser",
		Password:     "secret123",
		Privileges:   []string{"SELECT", "INSERT", "UPDATE"},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateDatabaseUserWorkflowTestSuite) TestGetUserFails_SetsStatusFailed() {
	userID := "test-dbuser-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(CreateDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateDatabaseUserWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	userID := "test-dbuser-3"
	databaseID := "test-database-3"
	shardID := "test-shard-3"

	dbUser := model.DatabaseUser{
		ID:         userID,
		DatabaseID: databaseID,
		Username:   "appuser",
		Password:   "secret123",
		Privileges: []string{"ALL"},
	}
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(&activity.DatabaseUserContext{
		User:     dbUser,
		Database: database,
		Nodes:    nodes,
	}, nil)
	s.env.OnActivity("CreateDatabaseUser", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(CreateDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateDatabaseUserWorkflowTestSuite) TestGetDatabaseFails_SetsStatusFailed() {
	userID := "test-dbuser-4"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(CreateDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UpdateDatabaseUserWorkflow ----------

type UpdateDatabaseUserWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateDatabaseUserWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UpdateDatabaseUserWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateDatabaseUserWorkflowTestSuite) TestSuccess() {
	userID := "test-dbuser-1"
	databaseID := "test-database-1"
	shardID := "test-shard-1"

	dbUser := model.DatabaseUser{
		ID:         userID,
		DatabaseID: databaseID,
		Username:   "appuser",
		Password:   "newsecret456",
		Privileges: []string{"SELECT", "INSERT"},
	}
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(&activity.DatabaseUserContext{
		User:     dbUser,
		Database: database,
		Nodes:    nodes,
	}, nil)
	s.env.OnActivity("UpdateDatabaseUser", mock.Anything, activity.UpdateDatabaseUserParams{
		DatabaseName: "mydb",
		Username:     "appuser",
		Password:     "newsecret456",
		Privileges:   []string{"SELECT", "INSERT"},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateDatabaseUserWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	userID := "test-dbuser-2"
	databaseID := "test-database-2"
	shardID := "test-shard-2"

	dbUser := model.DatabaseUser{
		ID:         userID,
		DatabaseID: databaseID,
		Username:   "appuser",
		Password:   "newsecret456",
		Privileges: []string{"SELECT"},
	}
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(&activity.DatabaseUserContext{
		User:     dbUser,
		Database: database,
		Nodes:    nodes,
	}, nil)
	s.env.OnActivity("UpdateDatabaseUser", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(UpdateDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UpdateDatabaseUserWorkflowTestSuite) TestGetUserFails_SetsStatusFailed() {
	userID := "test-dbuser-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(UpdateDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteDatabaseUserWorkflow ----------

type DeleteDatabaseUserWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteDatabaseUserWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteDatabaseUserWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteDatabaseUserWorkflowTestSuite) TestSuccess() {
	userID := "test-dbuser-1"
	databaseID := "test-database-1"
	shardID := "test-shard-1"

	dbUser := model.DatabaseUser{
		ID:         userID,
		DatabaseID: databaseID,
		Username:   "appuser",
		Password:   "secret123",
	}
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(&activity.DatabaseUserContext{
		User:     dbUser,
		Database: database,
		Nodes:    nodes,
	}, nil)
	s.env.OnActivity("DeleteDatabaseUser", mock.Anything, "mydb", "appuser").Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteDatabaseUserWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	userID := "test-dbuser-2"
	databaseID := "test-database-2"
	shardID := "test-shard-2"

	dbUser := model.DatabaseUser{
		ID:         userID,
		DatabaseID: databaseID,
		Username:   "appuser",
		Password:   "secret123",
	}
	database := model.Database{
		ID:      databaseID,
		Name:    "mydb",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(&activity.DatabaseUserContext{
		User:     dbUser,
		Database: database,
		Nodes:    nodes,
	}, nil)
	s.env.OnActivity("DeleteDatabaseUser", mock.Anything, "mydb", "appuser").Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteDatabaseUserWorkflowTestSuite) TestGetUserFails_SetsStatusFailed() {
	userID := "test-dbuser-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteDatabaseUserWorkflowTestSuite) TestGetDatabaseFails_SetsStatusFailed() {
	userID := "test-dbuser-4"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "database_users", ID: userID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseUserContext", mock.Anything, userID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("database_users", userID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteDatabaseUserWorkflow, userID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateDatabaseUserWorkflow(t *testing.T) {
	suite.Run(t, new(CreateDatabaseUserWorkflowTestSuite))
}

func TestUpdateDatabaseUserWorkflow(t *testing.T) {
	suite.Run(t, new(UpdateDatabaseUserWorkflowTestSuite))
}

func TestDeleteDatabaseUserWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteDatabaseUserWorkflowTestSuite))
}
