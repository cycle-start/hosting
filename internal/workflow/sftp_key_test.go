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

// ---------- AddSFTPKeyWorkflow ----------

type AddSFTPKeyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *AddSFTPKeyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *AddSFTPKeyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *AddSFTPKeyWorkflowTestSuite) TestSuccess() {
	keyID := "test-key-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SFTPKey{
		ID:        keyID,
		TenantID:  tenantID,
		Name:      "my-key",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@test",
		Status:    model.StatusPending,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSFTPKeysByTenant", mock.Anything, tenantID).Return([]model.SFTPKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSFTPKeys", mock.Anything, activity.SyncSFTPKeysParams{
		TenantName: tenantID,
		PublicKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@test"},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(AddSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *AddSFTPKeyWorkflowTestSuite) TestWithExistingActiveKeys() {
	keyID := "test-key-2"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SFTPKey{
		ID:        keyID,
		TenantID:  tenantID,
		Name:      "second-key",
		PublicKey: "ssh-rsa AAAAB3... second@test",
		Status:    model.StatusPending,
	}
	existingKey := model.SFTPKey{
		ID:        "existing-key-1",
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3... first@test",
		Status:    model.StatusActive,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSFTPKeysByTenant", mock.Anything, tenantID).Return([]model.SFTPKey{existingKey}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSFTPKeys", mock.Anything, activity.SyncSFTPKeysParams{
		TenantName: tenantID,
		PublicKeys: []string{"ssh-ed25519 AAAAC3... first@test", "ssh-rsa AAAAB3... second@test"},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(AddSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *AddSFTPKeyWorkflowTestSuite) TestGetKeyFails_SetsStatusFailed() {
	keyID := "test-key-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("sftp_keys", keyID)).Return(nil)

	s.env.ExecuteWorkflow(AddSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *AddSFTPKeyWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	keyID := "test-key-4"
	tenantID := "test-tenant-1"
	key := model.SFTPKey{
		ID:       keyID,
		TenantID: tenantID,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		// ShardID is nil
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("sftp_keys", keyID)).Return(nil)
	s.env.ExecuteWorkflow(AddSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *AddSFTPKeyWorkflowTestSuite) TestSyncFails_SetsStatusFailed() {
	keyID := "test-key-5"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SFTPKey{
		ID:        keyID,
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3...",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSFTPKeysByTenant", mock.Anything, tenantID).Return([]model.SFTPKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSFTPKeys", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("sftp_keys", keyID)).Return(nil)

	s.env.ExecuteWorkflow(AddSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *AddSFTPKeyWorkflowTestSuite) TestSetProvisioningFails() {
	keyID := "test-key-6"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(AddSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- RemoveSFTPKeyWorkflow ----------

type RemoveSFTPKeyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *RemoveSFTPKeyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *RemoveSFTPKeyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *RemoveSFTPKeyWorkflowTestSuite) TestSuccess() {
	keyID := "test-key-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SFTPKey{
		ID:        keyID,
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3...",
		Status:    model.StatusDeleting,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSFTPKeysByTenant", mock.Anything, tenantID).Return([]model.SFTPKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSFTPKeys", mock.Anything, activity.SyncSFTPKeysParams{
		TenantName: tenantID,
		PublicKeys: []string{},
	}).Return(nil)

	s.env.ExecuteWorkflow(RemoveSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *RemoveSFTPKeyWorkflowTestSuite) TestWithRemainingKeys() {
	keyID := "test-key-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SFTPKey{
		ID:        keyID,
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3...",
		Status:    model.StatusDeleting,
	}
	remainingKey := model.SFTPKey{
		ID:        "other-key",
		TenantID:  tenantID,
		PublicKey: "ssh-rsa AAAAB3... other@test",
		Status:    model.StatusActive,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSFTPKeysByTenant", mock.Anything, tenantID).Return([]model.SFTPKey{remainingKey}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSFTPKeys", mock.Anything, activity.SyncSFTPKeysParams{
		TenantName: tenantID,
		PublicKeys: []string{"ssh-rsa AAAAB3... other@test"},
	}).Return(nil)

	s.env.ExecuteWorkflow(RemoveSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *RemoveSFTPKeyWorkflowTestSuite) TestGetKeyFails_SetsStatusFailed() {
	keyID := "test-key-2"

	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("sftp_keys", keyID)).Return(nil)

	s.env.ExecuteWorkflow(RemoveSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *RemoveSFTPKeyWorkflowTestSuite) TestNoShard() {
	keyID := "test-key-3"
	tenantID := "test-tenant-1"
	key := model.SFTPKey{
		ID:       keyID,
		TenantID: tenantID,
		Status:   model.StatusDeleting,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		// ShardID is nil
	}

	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)

	s.env.ExecuteWorkflow(RemoveSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *RemoveSFTPKeyWorkflowTestSuite) TestSyncFails() {
	keyID := "test-key-4"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SFTPKey{
		ID:       keyID,
		TenantID: tenantID,
		Status:   model.StatusDeleting,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetSFTPKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "sftp_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSFTPKeysByTenant", mock.Anything, tenantID).Return([]model.SFTPKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSFTPKeys", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))

	s.env.ExecuteWorkflow(RemoveSFTPKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestAddSFTPKeyWorkflow(t *testing.T) {
	suite.Run(t, new(AddSFTPKeyWorkflowTestSuite))
}

func TestRemoveSFTPKeyWorkflow(t *testing.T) {
	suite.Run(t, new(RemoveSFTPKeyWorkflowTestSuite))
}
