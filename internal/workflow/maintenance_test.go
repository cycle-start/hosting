package workflow

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/model"
)

// ---------- CleanupAuditLogsWorkflow ----------

type CleanupAuditLogsWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CleanupAuditLogsWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CleanupAuditLogsWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CleanupAuditLogsWorkflowTestSuite) TestSuccess() {
	s.env.OnActivity("DeleteOldAuditLogs", mock.Anything, 90).Return(int64(42), nil)

	s.env.ExecuteWorkflow(CleanupAuditLogsWorkflow, 90)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CleanupAuditLogsWorkflowTestSuite) TestDeleteFails() {
	s.env.OnActivity("DeleteOldAuditLogs", mock.Anything, 90).Return(int64(0), fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(CleanupAuditLogsWorkflow, 90)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- CleanupOldBackupsWorkflow ----------

type CleanupOldBackupsWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CleanupOldBackupsWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CleanupOldBackupsWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CleanupOldBackupsWorkflowTestSuite) TestSuccess() {
	now := time.Now()
	oldBackups := []model.Backup{
		{ID: "backup-1", TenantID: "tenant-1", CreatedAt: now.Add(-60 * 24 * time.Hour)},
		{ID: "backup-2", TenantID: "tenant-2", CreatedAt: now.Add(-45 * 24 * time.Hour)},
	}

	s.env.OnActivity("GetOldBackups", mock.Anything, 30).Return(oldBackups, nil)
	s.env.OnWorkflow(DeleteBackupWorkflow, mock.Anything, "backup-1").Return(nil)
	s.env.OnWorkflow(DeleteBackupWorkflow, mock.Anything, "backup-2").Return(nil)

	s.env.ExecuteWorkflow(CleanupOldBackupsWorkflow, 30)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CleanupOldBackupsWorkflowTestSuite) TestNoBackups() {
	s.env.OnActivity("GetOldBackups", mock.Anything, 30).Return([]model.Backup{}, nil)

	s.env.ExecuteWorkflow(CleanupOldBackupsWorkflow, 30)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CleanupOldBackupsWorkflowTestSuite) TestDeleteChildFails_ContinuesOthers() {
	now := time.Now()
	oldBackups := []model.Backup{
		{ID: "backup-1", TenantID: "tenant-1", CreatedAt: now.Add(-60 * 24 * time.Hour)},
		{ID: "backup-2", TenantID: "tenant-2", CreatedAt: now.Add(-45 * 24 * time.Hour)},
	}

	s.env.OnActivity("GetOldBackups", mock.Anything, 30).Return(oldBackups, nil)
	s.env.OnWorkflow(DeleteBackupWorkflow, mock.Anything, "backup-1").Return(fmt.Errorf("delete failed"))
	s.env.OnWorkflow(DeleteBackupWorkflow, mock.Anything, "backup-2").Return(nil)

	s.env.ExecuteWorkflow(CleanupOldBackupsWorkflow, 30)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CleanupOldBackupsWorkflowTestSuite) TestGetOldBackupsFails() {
	s.env.OnActivity("GetOldBackups", mock.Anything, 30).Return(nil, fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(CleanupOldBackupsWorkflow, 30)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCleanupAuditLogsWorkflow(t *testing.T) {
	suite.Run(t, new(CleanupAuditLogsWorkflowTestSuite))
}

func TestCleanupOldBackupsWorkflow(t *testing.T) {
	suite.Run(t, new(CleanupOldBackupsWorkflowTestSuite))
}
