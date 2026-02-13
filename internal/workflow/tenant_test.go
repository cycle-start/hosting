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

// ---------- CreateTenantWorkflow ----------

type CreateTenantWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateTenantWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateTenantWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateTenantWorkflowTestSuite) TestSuccess() {
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{
		ID:          tenantID,
			UID:         5001,
		ShardID:     &shardID,
		SFTPEnabled: true,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateTenant", mock.Anything, activity.CreateTenantParams{
		ID:          tenantID,
			UID:         5001,
		SFTPEnabled: true,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateTenantWorkflowTestSuite) TestGetTenantFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(nil, fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-3"
	shardID := "test-shard-3"
	tenant := model.Tenant{
		ID:          tenantID,
			UID:         5001,
		ShardID:     &shardID,
		SFTPEnabled: false,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateTenant", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateTenantWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	tenantID := "test-tenant-no-shard"
	tenant := model.Tenant{
		ID:   tenantID,
		UID:  5001,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateTenantWorkflowTestSuite) TestSetProvisioningFails() {
	tenantID := "test-tenant-4"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))
	s.env.ExecuteWorkflow(CreateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UpdateTenantWorkflow ----------

type UpdateTenantWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateTenantWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UpdateTenantWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateTenantWorkflowTestSuite) TestSuccess() {
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{
		ID:          tenantID,
			UID:         5001,
		ShardID:     &shardID,
		SFTPEnabled: true,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("UpdateTenant", mock.Anything, activity.UpdateTenantParams{
		ID:          tenantID,
			UID:         5001,
		SFTPEnabled: true,
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"
	tenant := model.Tenant{
		ID:          tenantID,
			UID:         5001,
		ShardID:     &shardID,
		SFTPEnabled: true,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("UpdateTenant", mock.Anything, activity.UpdateTenantParams{
		ID:          tenantID,
			UID:         5001,
		SFTPEnabled: true,
	}).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- SuspendTenantWorkflow ----------

type SuspendTenantWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *SuspendTenantWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *SuspendTenantWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *SuspendTenantWorkflowTestSuite) TestSuccess() {
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{
		ID:      tenantID,
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SuspendTenant", mock.Anything, tenantID).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusSuspended,
	}).Return(nil)
	s.env.ExecuteWorkflow(SuspendTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *SuspendTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"
	tenant := model.Tenant{
		ID:      tenantID,
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SuspendTenant", mock.Anything, tenantID).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(SuspendTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *SuspendTenantWorkflowTestSuite) TestGetTenantFails() {
	tenantID := "test-tenant-3"

	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(nil, fmt.Errorf("db error"))
	s.env.ExecuteWorkflow(SuspendTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UnsuspendTenantWorkflow ----------

type UnsuspendTenantWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UnsuspendTenantWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UnsuspendTenantWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UnsuspendTenantWorkflowTestSuite) TestSuccess() {
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{
		ID:      tenantID,
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("UnsuspendTenant", mock.Anything, tenantID).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(UnsuspendTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UnsuspendTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"
	tenant := model.Tenant{
		ID:      tenantID,
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("UnsuspendTenant", mock.Anything, tenantID).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UnsuspendTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteTenantWorkflow ----------

type DeleteTenantWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteTenantWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteTenantWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteTenantWorkflowTestSuite) TestSuccess() {
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	tenant := model.Tenant{
		ID:      tenantID,
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteTenant", mock.Anything, tenantID).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"
	tenant := model.Tenant{
		ID:      tenantID,
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteTenant", mock.Anything, tenantID).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteTenantWorkflowTestSuite) TestGetTenantFails_SetsStatusFailed() {
	tenantID := "test-tenant-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(nil, fmt.Errorf("db error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateTenantWorkflow(t *testing.T) {
	suite.Run(t, new(CreateTenantWorkflowTestSuite))
}

func TestUpdateTenantWorkflow(t *testing.T) {
	suite.Run(t, new(UpdateTenantWorkflowTestSuite))
}

func TestSuspendTenantWorkflow(t *testing.T) {
	suite.Run(t, new(SuspendTenantWorkflowTestSuite))
}

func TestUnsuspendTenantWorkflow(t *testing.T) {
	suite.Run(t, new(UnsuspendTenantWorkflowTestSuite))
}

func TestDeleteTenantWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteTenantWorkflowTestSuite))
}
