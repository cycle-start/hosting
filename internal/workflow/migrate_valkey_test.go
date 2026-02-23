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

// ---------- MigrateValkeyInstanceWorkflow ----------

type MigrateValkeyInstanceWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) TestSuccess() {
	instanceID := "test-valkey-1"
	sourceShardID := "source-shard-1"
	targetShardID := "target-shard-1"

	instance := model.ValkeyInstance{
		ID:          instanceID,
		ShardID:     &sourceShardID,
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}
	sourceNodes := []model.Node{{ID: "source-node-1"}}
	targetNodes := []model.Node{{ID: "target-node-1"}}
	users := []model.ValkeyUser{
		{
			ID:               "vuser-1",
			ValkeyInstanceID: instanceID,
			Username:         "appuser",
			Password:         "userpass",
			Privileges:       []string{"+@read", "+@write"},
			KeyPattern:       "~app:*",
		},
	}

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.rdb", instance.ID)

	// Set provisioning.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)

	// Get instance.
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)

	// Get source and target nodes.
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)

	// Create instance on target.
	s.env.OnActivity("CreateValkeyInstance", mock.Anything, activity.CreateValkeyInstanceParams{
		Name:        instanceID,
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}).Return(nil)

	// Dump on source.
	s.env.OnActivity("DumpValkeyData", mock.Anything, activity.DumpValkeyDataParams{
		Name:     instanceID,
		Port:     6379,
		Password: "valkeypass",
		DumpPath: dumpPath,
	}).Return(nil)

	// Import on target.
	s.env.OnActivity("ImportValkeyData", mock.Anything, activity.ImportValkeyDataParams{
		Name:     instanceID,
		Port:     6379,
		DumpPath: dumpPath,
	}).Return(nil)

	// List and migrate users.
	s.env.OnActivity("ListValkeyUsersByInstanceID", mock.Anything, instanceID).Return(users, nil)
	s.env.OnActivity("CreateValkeyUser", mock.Anything, activity.CreateValkeyUserParams{
		InstanceName: instanceID,
		Port:         6379,
		Username:     "appuser",
		Password:     "userpass",
		Privileges:   []string{"+@read", "+@write"},
		KeyPattern:   "~app:*",
	}).Return(nil)

	// Update shard assignment.
	s.env.OnActivity("UpdateValkeyInstanceShardID", mock.Anything, instanceID, targetShardID).Return(nil)

	// Cleanup (best effort).
	s.env.OnActivity("DeleteValkeyInstance", mock.Anything, activity.DeleteValkeyInstanceParams{
		Name: instanceID,
		Port: 6379,
	}).Return(nil)
	s.env.OnActivity("CleanupMigrateFile", mock.Anything, dumpPath).Return(nil)

	// Set active.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(MigrateValkeyInstanceWorkflow, MigrateValkeyInstanceParams{
		InstanceID:    instanceID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) TestSuccessNoUsers() {
	instanceID := "test-valkey-2"
	sourceShardID := "source-shard-2"
	targetShardID := "target-shard-2"

	instance := model.ValkeyInstance{
		ID:          instanceID,
		ShardID:     &sourceShardID,
		Port:        6380,
		Password:    "valkeypass2",
		MaxMemoryMB: 128,
	}
	sourceNodes := []model.Node{{ID: "source-node-2"}}
	targetNodes := []model.Node{{ID: "target-node-2"}}

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.rdb", instance.ID)

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)
	s.env.OnActivity("CreateValkeyInstance", mock.Anything, activity.CreateValkeyInstanceParams{
		Name:        instanceID,
		Port:        6380,
		Password:    "valkeypass2",
		MaxMemoryMB: 128,
	}).Return(nil)
	s.env.OnActivity("DumpValkeyData", mock.Anything, activity.DumpValkeyDataParams{
		Name:     instanceID,
		Port:     6380,
		Password: "valkeypass2",
		DumpPath: dumpPath,
	}).Return(nil)
	s.env.OnActivity("ImportValkeyData", mock.Anything, activity.ImportValkeyDataParams{
		Name:     instanceID,
		Port:     6380,
		DumpPath: dumpPath,
	}).Return(nil)

	// No users.
	var emptyUsers []model.ValkeyUser
	s.env.OnActivity("ListValkeyUsersByInstanceID", mock.Anything, instanceID).Return(emptyUsers, nil)

	s.env.OnActivity("UpdateValkeyInstanceShardID", mock.Anything, instanceID, targetShardID).Return(nil)
	s.env.OnActivity("DeleteValkeyInstance", mock.Anything, activity.DeleteValkeyInstanceParams{
		Name: instanceID,
		Port: 6380,
	}).Return(nil)
	s.env.OnActivity("CleanupMigrateFile", mock.Anything, dumpPath).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(MigrateValkeyInstanceWorkflow, MigrateValkeyInstanceParams{
		InstanceID:    instanceID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) TestGetInstanceFails_SetsStatusFailed() {
	instanceID := "test-valkey-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("valkey_instances", instanceID)).Return(nil)

	s.env.ExecuteWorkflow(MigrateValkeyInstanceWorkflow, MigrateValkeyInstanceParams{
		InstanceID:    instanceID,
		TargetShardID: "target-shard-3",
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	instanceID := "test-valkey-4"

	instance := model.ValkeyInstance{
		ID:          instanceID,
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("valkey_instances", instanceID)).Return(nil)
	s.env.ExecuteWorkflow(MigrateValkeyInstanceWorkflow, MigrateValkeyInstanceParams{
		InstanceID:    instanceID,
		TargetShardID: "target-shard-4",
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) TestDumpFails_SetsStatusFailed() {
	instanceID := "test-valkey-5"
	sourceShardID := "source-shard-5"
	targetShardID := "target-shard-5"

	instance := model.ValkeyInstance{
		ID:          instanceID,
		ShardID:     &sourceShardID,
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}
	sourceNodes := []model.Node{{ID: "source-node-5"}}
	targetNodes := []model.Node{{ID: "target-node-5"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)
	s.env.OnActivity("CreateValkeyInstance", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("DumpValkeyData", mock.Anything, mock.Anything).Return(fmt.Errorf("BGSAVE failed"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("valkey_instances", instanceID)).Return(nil)

	s.env.ExecuteWorkflow(MigrateValkeyInstanceWorkflow, MigrateValkeyInstanceParams{
		InstanceID:    instanceID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) TestImportFails_SetsStatusFailed() {
	instanceID := "test-valkey-6"
	sourceShardID := "source-shard-6"
	targetShardID := "target-shard-6"

	instance := model.ValkeyInstance{
		ID:          instanceID,
		ShardID:     &sourceShardID,
		Port:        6379,
		Password:    "valkeypass",
		MaxMemoryMB: 256,
	}
	sourceNodes := []model.Node{{ID: "source-node-6"}}
	targetNodes := []model.Node{{ID: "target-node-6"}}

	dumpPath := fmt.Sprintf("/var/backups/hosting/migrate/%s.rdb", instance.ID)

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetValkeyInstanceByID", mock.Anything, instanceID).Return(&instance, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, sourceShardID).Return(sourceNodes, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, targetShardID).Return(targetNodes, nil)
	s.env.OnActivity("CreateValkeyInstance", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("DumpValkeyData", mock.Anything, activity.DumpValkeyDataParams{
		Name:     instanceID,
		Port:     6379,
		Password: "valkeypass",
		DumpPath: dumpPath,
	}).Return(nil)
	s.env.OnActivity("ImportValkeyData", mock.Anything, mock.Anything).Return(fmt.Errorf("import failed"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("valkey_instances", instanceID)).Return(nil)

	s.env.ExecuteWorkflow(MigrateValkeyInstanceWorkflow, MigrateValkeyInstanceParams{
		InstanceID:    instanceID,
		TargetShardID: targetShardID,
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *MigrateValkeyInstanceWorkflowTestSuite) TestSetProvisioningFails() {
	instanceID := "test-valkey-7"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "valkey_instances", ID: instanceID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(MigrateValkeyInstanceWorkflow, MigrateValkeyInstanceParams{
		InstanceID:    instanceID,
		TargetShardID: "target-shard-7",
	})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestMigrateValkeyInstanceWorkflow(t *testing.T) {
	suite.Run(t, new(MigrateValkeyInstanceWorkflowTestSuite))
}
