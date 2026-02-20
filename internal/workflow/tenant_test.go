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
	shardIdx := 1
	tenant := model.Tenant{
		ID:          tenantID,
		Name:        "t_test123456",
		BrandID:     "test-brand",
		UID:         5001,
		ShardID:     &shardID,
		SFTPEnabled: true,
	}
	nodes := []model.Node{
		{ID: "node-1", ClusterID: "dev-1", ShardIndex: &shardIdx},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateTenant", mock.Anything, activity.CreateTenantParams{
		ID:          tenantID,
		Name:        "t_test123456",
		UID:         5001,
		SFTPEnabled: true,
	}).Return(nil)
	s.env.OnActivity("SyncSSHConfig", mock.Anything, activity.SyncSSHConfigParams{
		TenantName: "t_test123456", SFTPEnabled: true,
	}).Return(nil)
	s.env.OnActivity("ConfigureTenantAddresses", mock.Anything, activity.ConfigureTenantAddressesParams{
		TenantName: "t_test123456", TenantUID: 5001, ClusterID: "dev-1", NodeShardIdx: 1,
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
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
	s.env.ExecuteWorkflow(CreateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-3"
	shardID := "test-shard-3"
	tenant := model.Tenant{
		ID:          tenantID,
		Name:        "t_test123456",
		BrandID:     "test-brand",
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
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
	s.env.ExecuteWorkflow(CreateTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateTenantWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	tenantID := "test-tenant-no-shard"
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		UID:     5001,
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
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
		Name:        "t_test123456",
		BrandID:     "test-brand",
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
		Name:        "t_test123456",
		UID:         5001,
		SFTPEnabled: true,
	}).Return(nil)
	s.env.OnActivity("SyncSSHConfig", mock.Anything, activity.SyncSSHConfigParams{
		TenantName: "t_test123456", SFTPEnabled: true,
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
		Name:        "t_test123456",
		BrandID:     "test-brand",
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
		Name:        "t_test123456",
			UID:         5001,
		SFTPEnabled: true,
	}).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
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
		ID:            tenantID,
		Name:          "t_test123456",
		BrandID:       "test-brand",
		UID:           5001,
		ShardID:       &shardID,
		SuspendReason: "abuse",
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SuspendTenant", mock.Anything, "t_test123456").Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusSuspended,
	}).Return(nil)
	// Cascade activities — return empty lists (no children).
	s.env.OnActivity("ListWebrootsByTenantID", mock.Anything, tenantID).Return([]model.Webroot{}, nil)
	s.env.OnActivity("ListDatabasesByTenantID", mock.Anything, tenantID).Return([]model.Database{}, nil)
	s.env.OnActivity("ListValkeyInstancesByTenantID", mock.Anything, tenantID).Return([]model.ValkeyInstance{}, nil)
	s.env.OnActivity("ListS3BucketsByTenantID", mock.Anything, tenantID).Return([]model.S3Bucket{}, nil)
	s.env.OnActivity("ListZonesByTenantID", mock.Anything, tenantID).Return([]model.Zone{}, nil)
	s.env.ExecuteWorkflow(SuspendTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *SuspendTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SuspendTenant", mock.Anything, "t_test123456").Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
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
		Name:    "t_test123456",
		BrandID: "test-brand",
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
	s.env.OnActivity("UnsuspendTenant", mock.Anything, "t_test123456").Return(nil)
	s.env.OnActivity("UnsuspendResource", mock.Anything, activity.SuspendResourceParams{
		Table: "tenants", ID: tenantID,
	}).Return(nil)
	// Cascade activities — return empty lists (no children).
	s.env.OnActivity("ListWebrootsByTenantID", mock.Anything, tenantID).Return([]model.Webroot{}, nil)
	s.env.OnActivity("ListDatabasesByTenantID", mock.Anything, tenantID).Return([]model.Database{}, nil)
	s.env.OnActivity("ListValkeyInstancesByTenantID", mock.Anything, tenantID).Return([]model.ValkeyInstance{}, nil)
	s.env.OnActivity("ListS3BucketsByTenantID", mock.Anything, tenantID).Return([]model.S3Bucket{}, nil)
	s.env.OnActivity("ListZonesByTenantID", mock.Anything, tenantID).Return([]model.Zone{}, nil)
	s.env.ExecuteWorkflow(UnsuspendTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UnsuspendTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
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
	s.env.OnActivity("UnsuspendTenant", mock.Anything, "t_test123456").Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
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

// mockDeleteTenantPhase1 sets up mocks for the Phase 1 cross-shard resource listing
// (returning empty lists so no child workflows are spawned).
func mockDeleteTenantPhase1Empty(env *testsuite.TestWorkflowEnvironment, tenantID string) {
	env.OnActivity("ListDatabasesByTenantID", mock.Anything, tenantID).Return([]model.Database{}, nil)
	env.OnActivity("ListValkeyInstancesByTenantID", mock.Anything, tenantID).Return([]model.ValkeyInstance{}, nil)
	env.OnActivity("ListS3BucketsByTenantID", mock.Anything, tenantID).Return([]model.S3Bucket{}, nil)
	env.OnActivity("ListZonesByTenantID", mock.Anything, tenantID).Return([]model.Zone{}, nil)
	env.OnActivity("ListEmailAccountsByTenantID", mock.Anything, tenantID).Return([]model.EmailAccount{}, nil)
}

func (s *DeleteTenantWorkflowTestSuite) TestSuccess() {
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	shardIdx := 1
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1", ClusterID: "dev-1", ShardIndex: &shardIdx},
	}

	// Set deleting.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)

	// Phase 1: empty cross-shard resources.
	mockDeleteTenantPhase1Empty(s.env, tenantID)

	// Phase 2: web-node cleanup.
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("RemoveTenantAddresses", mock.Anything, activity.ConfigureTenantAddressesParams{
		TenantName: "t_test123456", TenantUID: 5001, ClusterID: "dev-1", NodeShardIdx: 1,
	}).Return(nil)
	s.env.OnActivity("RemoveSSHConfig", mock.Anything, "t_test123456").Return(nil)
	s.env.OnActivity("DeleteTenant", mock.Anything, "t_test123456").Return(nil)

	// Phase 3: convergence (child workflow).
	s.env.OnWorkflow(ConvergeShardWorkflow, mock.Anything, ConvergeShardParams{ShardID: shardID}).Return(nil)

	// Phase 4: DB row cleanup.
	s.env.OnActivity("DeleteTenantDBRows", mock.Anything, tenantID).Return(nil)

	s.env.ExecuteWorkflow(DeleteTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteTenantWorkflowTestSuite) TestWithCrossShardResources() {
	tenantID := "test-tenant-cross"
	shardID := "test-shard-1"
	shardIdx := 1
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1", ClusterID: "dev-1", ShardIndex: &shardIdx},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)

	// Phase 1: some cross-shard resources exist.
	s.env.OnActivity("ListDatabasesByTenantID", mock.Anything, tenantID).Return([]model.Database{
		{ID: "db-1", TenantID: &tenantID},
	}, nil)
	s.env.OnActivity("ListValkeyInstancesByTenantID", mock.Anything, tenantID).Return([]model.ValkeyInstance{
		{ID: "vi-1", TenantID: &tenantID},
	}, nil)
	s.env.OnActivity("ListS3BucketsByTenantID", mock.Anything, tenantID).Return([]model.S3Bucket{}, nil)
	s.env.OnActivity("ListZonesByTenantID", mock.Anything, tenantID).Return([]model.Zone{
		{ID: "zone-1", TenantID: &tenantID},
	}, nil)
	s.env.OnActivity("ListEmailAccountsByTenantID", mock.Anything, tenantID).Return([]model.EmailAccount{}, nil)

	// Phase 1 child workflows.
	s.env.OnWorkflow(DeleteDatabaseWorkflow, mock.Anything, "db-1").Return(nil)
	s.env.OnWorkflow(DeleteValkeyInstanceWorkflow, mock.Anything, "vi-1").Return(nil)
	s.env.OnWorkflow(DeleteZoneWorkflow, mock.Anything, "zone-1").Return(nil)

	// Phase 2: web-node cleanup.
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("RemoveTenantAddresses", mock.Anything, activity.ConfigureTenantAddressesParams{
		TenantName: "t_test123456", TenantUID: 5001, ClusterID: "dev-1", NodeShardIdx: 1,
	}).Return(nil)
	s.env.OnActivity("RemoveSSHConfig", mock.Anything, "t_test123456").Return(nil)
	s.env.OnActivity("DeleteTenant", mock.Anything, "t_test123456").Return(nil)

	// Phase 3 + 4.
	s.env.OnWorkflow(ConvergeShardWorkflow, mock.Anything, ConvergeShardParams{ShardID: shardID}).Return(nil)
	s.env.OnActivity("DeleteTenantDBRows", mock.Anything, tenantID).Return(nil)

	s.env.ExecuteWorkflow(DeleteTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteTenantWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"
	shardIdx := 1
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1", ClusterID: "dev-1", ShardIndex: &shardIdx},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)

	// Phase 1: empty.
	mockDeleteTenantPhase1Empty(s.env, tenantID)

	// Phase 2: node cleanup fails.
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("RemoveTenantAddresses", mock.Anything, activity.ConfigureTenantAddressesParams{
		TenantName: "t_test123456", TenantUID: 5001, ClusterID: "dev-1", NodeShardIdx: 1,
	}).Return(nil)
	s.env.OnActivity("RemoveSSHConfig", mock.Anything, "t_test123456").Return(nil)
	s.env.OnActivity("DeleteTenant", mock.Anything, "t_test123456").Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
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
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)
	s.env.ExecuteWorkflow(DeleteTenantWorkflow, tenantID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteTenantWorkflowTestSuite) TestDBRowCleanupFails_SetsStatusFailed() {
	tenantID := "test-tenant-4"
	shardID := "test-shard-4"
	shardIdx := 1
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1", ClusterID: "dev-1", ShardIndex: &shardIdx},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "tenants", ID: tenantID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	mockDeleteTenantPhase1Empty(s.env, tenantID)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("RemoveTenantAddresses", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("RemoveSSHConfig", mock.Anything, "t_test123456").Return(nil)
	s.env.OnActivity("DeleteTenant", mock.Anything, "t_test123456").Return(nil)
	s.env.OnWorkflow(ConvergeShardWorkflow, mock.Anything, ConvergeShardParams{ShardID: shardID}).Return(nil)
	s.env.OnActivity("DeleteTenantDBRows", mock.Anything, tenantID).Return(fmt.Errorf("FK violation"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("tenants", tenantID)).Return(nil)

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
