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

// ---------- CreateBackupWorkflow ----------

type CreateBackupWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateBackupWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateBackupWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateBackupWorkflowTestSuite) TestSuccess_WebBackup() {
	backupID := "test-backup-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"

	backup := model.Backup{
		ID:         backupID,
		TenantID:   tenantID,
		Type:       model.BackupTypeWeb,
		SourceID:   "test-webroot-1",
		SourceName: "mysite",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}
	webroot := model.Webroot{
		ID:   "test-webroot-1",
		Name: "mysite",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-1").Return(&webroot, nil)
	s.env.OnActivity("CreateWebBackup", mock.Anything, mock.Anything).Return(&activity.BackupResult{
		StoragePath: "/var/backups/hosting/t_test123456/test-backup-1.tar.gz",
		SizeBytes:   2048,
	}, nil)
	s.env.OnActivity("UpdateBackupResult", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateBackupWorkflowTestSuite) TestSuccess_DatabaseBackup() {
	backupID := "test-backup-2"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"

	backup := model.Backup{
		ID:         backupID,
		TenantID:   tenantID,
		Type:       model.BackupTypeDatabase,
		SourceID:   "test-database-1",
		SourceName: "mydb",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("CreateMySQLBackup", mock.Anything, mock.Anything).Return(&activity.BackupResult{
		StoragePath: "/var/backups/hosting/t_test123456/test-backup-2.sql.gz",
		SizeBytes:   4096,
	}, nil)
	s.env.OnActivity("UpdateBackupResult", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateBackupWorkflowTestSuite) TestGetBackupFails_SetsStatusFailed() {
	backupID := "test-backup-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("backups", backupID)).Return(nil)

	s.env.ExecuteWorkflow(CreateBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateBackupWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	backupID := "test-backup-4"
	tenantID := "test-tenant-1"

	backup := model.Backup{
		ID:       backupID,
		TenantID: tenantID,
		Type:     model.BackupTypeWeb,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: nil, // no shard
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nil,
	}, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("backups", backupID)).Return(nil)
	s.env.ExecuteWorkflow(CreateBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateBackupWorkflowTestSuite) TestSetProvisioningFails() {
	backupID := "test-backup-5"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(CreateBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateBackupWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	backupID := "test-backup-6"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"

	backup := model.Backup{
		ID:         backupID,
		TenantID:   tenantID,
		Type:       model.BackupTypeWeb,
		SourceID:   "test-webroot-1",
		SourceName: "mysite",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}
	webroot := model.Webroot{
		ID:   "test-webroot-1",
		Name: "mysite",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-1").Return(&webroot, nil)
	s.env.OnActivity("CreateWebBackup", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("backups", backupID)).Return(nil)

	s.env.ExecuteWorkflow(CreateBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- RestoreBackupWorkflow ----------

type RestoreBackupWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *RestoreBackupWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *RestoreBackupWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *RestoreBackupWorkflowTestSuite) TestSuccess_WebRestore() {
	backupID := "test-backup-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	now := time.Now()

	backup := model.Backup{
		ID:          backupID,
		TenantID:    tenantID,
		Type:        model.BackupTypeWeb,
		SourceID:    "test-webroot-1",
		SourceName:  "mysite",
		StoragePath: "/var/backups/hosting/t_test123456/test-backup-1.tar.gz",
		SizeBytes:   2048,
		Status:      model.StatusActive,
		StartedAt:   &now,
		CompletedAt: &now,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}
	webroot := model.Webroot{
		ID:   "test-webroot-1",
		Name: "mysite",
	}

	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, "test-webroot-1").Return(&webroot, nil)
	s.env.OnActivity("RestoreWebBackup", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(RestoreBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *RestoreBackupWorkflowTestSuite) TestSuccess_DatabaseRestore() {
	backupID := "test-backup-2"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	now := time.Now()

	backup := model.Backup{
		ID:          backupID,
		TenantID:    tenantID,
		Type:        model.BackupTypeDatabase,
		SourceID:    "test-database-1",
		SourceName:  "mydb",
		StoragePath: "/var/backups/hosting/t_test123456/test-backup-2.sql.gz",
		SizeBytes:   4096,
		Status:      model.StatusActive,
		StartedAt:   &now,
		CompletedAt: &now,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("RestoreMySQLBackup", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(RestoreBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *RestoreBackupWorkflowTestSuite) TestGetBackupFails() {
	backupID := "test-backup-3"

	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(nil, fmt.Errorf("not found"))

	s.env.ExecuteWorkflow(RestoreBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *RestoreBackupWorkflowTestSuite) TestRestoreFails_SetsStatusFailed() {
	backupID := "test-backup-4"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	now := time.Now()

	backup := model.Backup{
		ID:          backupID,
		TenantID:    tenantID,
		Type:        model.BackupTypeDatabase,
		SourceID:    "test-database-1",
		SourceName:  "mydb",
		StoragePath: "/var/backups/hosting/t_test123456/test-backup-4.sql.gz",
		Status:      model.StatusActive,
		StartedAt:   &now,
		CompletedAt: &now,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("RestoreMySQLBackup", mock.Anything, mock.Anything).Return(fmt.Errorf("restore failed"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("backups", backupID)).Return(nil)

	s.env.ExecuteWorkflow(RestoreBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteBackupWorkflow ----------

type DeleteBackupWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteBackupWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteBackupWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteBackupWorkflowTestSuite) TestSuccess() {
	backupID := "test-backup-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	now := time.Now()

	backup := model.Backup{
		ID:          backupID,
		TenantID:    tenantID,
		Type:        model.BackupTypeWeb,
		SourceName:  "mysite",
		StoragePath: "/var/backups/hosting/t_test123456/test-backup-1.tar.gz",
		SizeBytes:   2048,
		Status:      model.StatusDeleting,
		StartedAt:   &now,
		CompletedAt: &now,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("DeleteBackupFile", mock.Anything, backup.StoragePath).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "backups", ID: backupID, Status: model.StatusDeleted,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteBackupWorkflowTestSuite) TestGetBackupFails_SetsStatusFailed() {
	backupID := "test-backup-2"

	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("backups", backupID)).Return(nil)

	s.env.ExecuteWorkflow(DeleteBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteBackupWorkflowTestSuite) TestDeleteFileFails_SetsStatusFailed() {
	backupID := "test-backup-3"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	now := time.Now()

	backup := model.Backup{
		ID:          backupID,
		TenantID:    tenantID,
		Type:        model.BackupTypeWeb,
		StoragePath: "/var/backups/hosting/t_test123456/test-backup-3.tar.gz",
		Status:      model.StatusDeleting,
		StartedAt:   &now,
		CompletedAt: &now,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetBackupContext", mock.Anything, backupID).Return(&activity.BackupContext{
		Backup: backup,
		Tenant: tenant,
		Nodes:  nodes,
	}, nil)
	s.env.OnActivity("DeleteBackupFile", mock.Anything, mock.Anything).Return(fmt.Errorf("file not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("backups", backupID)).Return(nil)

	s.env.ExecuteWorkflow(DeleteBackupWorkflow, backupID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateBackupWorkflow(t *testing.T) {
	suite.Run(t, new(CreateBackupWorkflowTestSuite))
}

func TestRestoreBackupWorkflow(t *testing.T) {
	suite.Run(t, new(RestoreBackupWorkflowTestSuite))
}

func TestDeleteBackupWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteBackupWorkflowTestSuite))
}
