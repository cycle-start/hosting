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

// ---------- MigrateDatabaseWorkflow ----------

type MigrateDatabaseWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *MigrateDatabaseWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *MigrateDatabaseWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *MigrateDatabaseWorkflowTestSuite) TestSuccess() {
	databaseID := "test-db-1"
	sourceShardID := "source-shard-1"
	targetShardID := "target-shard-1"

	database := model.Database{
		ID:      databaseID,
		ShardID: &sourceShardID,
	}
	sourceNodes := []model.Node{{ID: "source-node-1"}}
	targetNodes := []model.Node{{ID: "target-node-1"}}
	users := []model.DatabaseUser{
		{
			ID:         "user-1",
			DatabaseID: databaseID,
			Username:   "appuser",
			Password:   "secret123",
			Privileges: []string{"SELECT", "INSERT"},
		},
	}

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.sql.gz", database.ID)

	// Set provisioning.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)

	// Get database.
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)

	// Get source and target nodes.
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)

	// Create database on target.
	s.env.OnActivity("CreateDatabase", mock.Anything, databaseID).Return(nil)

	// Dump on source.
	s.env.OnActivity("DumpMySQLDatabase", mock.Anything, activity.DumpMySQLDatabaseParams{
		DatabaseName: databaseID,
		DumpPath:     dumpPath,
	}).Return(nil)

	// Import on target.
	s.env.OnActivity("ImportMySQLDatabase", mock.Anything, activity.ImportMySQLDatabaseParams{
		DatabaseName: databaseID,
		DumpPath:     dumpPath,
	}).Return(nil)

	// List and migrate users.
	s.env.OnActivity("ListDatabaseUsersByDatabaseID", mock.Anything, databaseID).Return(users, nil)
	s.env.OnActivity("CreateDatabaseUser", mock.Anything, activity.CreateDatabaseUserParams{
		DatabaseName: databaseID,
		Username:     "appuser",
		Password:     "secret123",
		Privileges:   []string{"SELECT", "INSERT"},
	}).Return(nil)

	// Update shard assignment.
	s.env.OnActivity("UpdateDatabaseShardID", mock.Anything, databaseID, targetShardID).Return(nil)

	// Cleanup (best effort).
	s.env.OnActivity("DeleteDatabase", mock.Anything, databaseID).Return(nil)
	s.env.OnActivity("CleanupMigrateFile", mock.Anything, dumpPath).Return(nil)

	// Set active.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(MigrateDatabaseWorkflow, MigrateDatabaseParams{
		DatabaseID:    databaseID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *MigrateDatabaseWorkflowTestSuite) TestSuccessNoUsers() {
	databaseID := "test-db-2"
	sourceShardID := "source-shard-2"
	targetShardID := "target-shard-2"

	database := model.Database{
		ID:      databaseID,
		ShardID: &sourceShardID,
	}
	sourceNodes := []model.Node{{ID: "source-node-2"}}
	targetNodes := []model.Node{{ID: "target-node-2"}}

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.sql.gz", database.ID)

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)
	s.env.OnActivity("CreateDatabase", mock.Anything, databaseID).Return(nil)
	s.env.OnActivity("DumpMySQLDatabase", mock.Anything, activity.DumpMySQLDatabaseParams{
		DatabaseName: databaseID,
		DumpPath:     dumpPath,
	}).Return(nil)
	s.env.OnActivity("ImportMySQLDatabase", mock.Anything, activity.ImportMySQLDatabaseParams{
		DatabaseName: databaseID,
		DumpPath:     dumpPath,
	}).Return(nil)

	// No users.
	var emptyUsers []model.DatabaseUser
	s.env.OnActivity("ListDatabaseUsersByDatabaseID", mock.Anything, databaseID).Return(emptyUsers, nil)

	s.env.OnActivity("UpdateDatabaseShardID", mock.Anything, databaseID, targetShardID).Return(nil)
	s.env.OnActivity("DeleteDatabase", mock.Anything, databaseID).Return(nil)
	s.env.OnActivity("CleanupMigrateFile", mock.Anything, dumpPath).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(MigrateDatabaseWorkflow, MigrateDatabaseParams{
		DatabaseID:    databaseID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *MigrateDatabaseWorkflowTestSuite) TestGetDatabaseFails_SetsStatusFailed() {
	databaseID := "test-db-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("databases", databaseID)).Return(nil)

	s.env.ExecuteWorkflow(MigrateDatabaseWorkflow, MigrateDatabaseParams{
		DatabaseID:    databaseID,
		TargetShardID: "target-shard-3",
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateDatabaseWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	databaseID := "test-db-4"

	database := model.Database{
		ID:   databaseID,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("databases", databaseID)).Return(nil)
	s.env.ExecuteWorkflow(MigrateDatabaseWorkflow, MigrateDatabaseParams{
		DatabaseID:    databaseID,
		TargetShardID: "target-shard-4",
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateDatabaseWorkflowTestSuite) TestDumpFails_SetsStatusFailed() {
	databaseID := "test-db-5"
	sourceShardID := "source-shard-5"
	targetShardID := "target-shard-5"

	database := model.Database{
		ID:      databaseID,
		ShardID: &sourceShardID,
	}
	sourceNodes := []model.Node{{ID: "source-node-5"}}
	targetNodes := []model.Node{{ID: "target-node-5"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)
	s.env.OnActivity("CreateDatabase", mock.Anything, databaseID).Return(nil)
	s.env.OnActivity("DumpMySQLDatabase", mock.Anything, mock.Anything).Return(fmt.Errorf("mysqldump failed"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("databases", databaseID)).Return(nil)

	s.env.ExecuteWorkflow(MigrateDatabaseWorkflow, MigrateDatabaseParams{
		DatabaseID:    databaseID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateDatabaseWorkflowTestSuite) TestImportFails_SetsStatusFailed() {
	databaseID := "test-db-6"
	sourceShardID := "source-shard-6"
	targetShardID := "target-shard-6"

	database := model.Database{
		ID:      databaseID,
		ShardID: &sourceShardID,
	}
	sourceNodes := []model.Node{{ID: "source-node-6"}}
	targetNodes := []model.Node{{ID: "target-node-6"}}

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.sql.gz", database.ID)

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDatabaseByID", mock.Anything, databaseID).Return(&database, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)
	s.env.OnActivity("CreateDatabase", mock.Anything, databaseID).Return(nil)
	s.env.OnActivity("DumpMySQLDatabase", mock.Anything, activity.DumpMySQLDatabaseParams{
		DatabaseName: databaseID,
		DumpPath:     dumpPath,
	}).Return(nil)
	s.env.OnActivity("ImportMySQLDatabase", mock.Anything, mock.Anything).Return(fmt.Errorf("import failed"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("databases", databaseID)).Return(nil)

	s.env.ExecuteWorkflow(MigrateDatabaseWorkflow, MigrateDatabaseParams{
		DatabaseID:    databaseID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateDatabaseWorkflowTestSuite) TestSetProvisioningFails() {
	databaseID := "test-db-7"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "databases", ID: databaseID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(MigrateDatabaseWorkflow, MigrateDatabaseParams{
		DatabaseID:    databaseID,
		TargetShardID: "target-shard-7",
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestMigrateDatabaseWorkflow(t *testing.T) {
	suite.Run(t, new(MigrateDatabaseWorkflowTestSuite))
}
