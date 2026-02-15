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

// ---------- AddSSHKeyWorkflow ----------

type AddSSHKeyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *AddSSHKeyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *AddSSHKeyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *AddSSHKeyWorkflowTestSuite) TestSuccess() {
	keyID := "test-key-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SSHKey{
		ID:        keyID,
		TenantID:  tenantID,
		Name:      "my-key",
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@test",
		Status:    model.StatusPending,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSSHKeysByTenant", mock.Anything, tenantID).Return([]model.SSHKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSSHKeys", mock.Anything, activity.SyncSSHKeysParams{
		TenantName: "t_test123456",
		PublicKeys: []string{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test@test"},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(AddSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *AddSSHKeyWorkflowTestSuite) TestWithExistingActiveKeys() {
	keyID := "test-key-2"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SSHKey{
		ID:        keyID,
		TenantID:  tenantID,
		Name:      "second-key",
		PublicKey: "ssh-rsa AAAAB3... second@test",
		Status:    model.StatusPending,
	}
	existingKey := model.SSHKey{
		ID:        "existing-key-1",
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3... first@test",
		Status:    model.StatusActive,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSSHKeysByTenant", mock.Anything, tenantID).Return([]model.SSHKey{existingKey}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSSHKeys", mock.Anything, activity.SyncSSHKeysParams{
		TenantName: "t_test123456",
		PublicKeys: []string{"ssh-ed25519 AAAAC3... first@test", "ssh-rsa AAAAB3... second@test"},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(AddSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *AddSSHKeyWorkflowTestSuite) TestGetKeyFails_SetsStatusFailed() {
	keyID := "test-key-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("ssh_keys", keyID)).Return(nil)

	s.env.ExecuteWorkflow(AddSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *AddSSHKeyWorkflowTestSuite) TestNoShard_SetsStatusFailed() {
	keyID := "test-key-4"
	tenantID := "test-tenant-1"
	key := model.SSHKey{
		ID:       keyID,
		TenantID: tenantID,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		// ShardID is nil
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("ssh_keys", keyID)).Return(nil)
	s.env.ExecuteWorkflow(AddSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *AddSSHKeyWorkflowTestSuite) TestSyncFails_SetsStatusFailed() {
	keyID := "test-key-5"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SSHKey{
		ID:        keyID,
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3...",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSSHKeysByTenant", mock.Anything, tenantID).Return([]model.SSHKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSSHKeys", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("ssh_keys", keyID)).Return(nil)

	s.env.ExecuteWorkflow(AddSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *AddSSHKeyWorkflowTestSuite) TestSetProvisioningFails() {
	keyID := "test-key-6"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(AddSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- RemoveSSHKeyWorkflow ----------

type RemoveSSHKeyWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *RemoveSSHKeyWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *RemoveSSHKeyWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *RemoveSSHKeyWorkflowTestSuite) TestSuccess() {
	keyID := "test-key-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SSHKey{
		ID:        keyID,
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3...",
		Status:    model.StatusDeleting,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSSHKeysByTenant", mock.Anything, tenantID).Return([]model.SSHKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSSHKeys", mock.Anything, activity.SyncSSHKeysParams{
		TenantName: "t_test123456",
		PublicKeys: []string{},
	}).Return(nil)

	s.env.ExecuteWorkflow(RemoveSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *RemoveSSHKeyWorkflowTestSuite) TestWithRemainingKeys() {
	keyID := "test-key-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SSHKey{
		ID:        keyID,
		TenantID:  tenantID,
		PublicKey: "ssh-ed25519 AAAAC3...",
		Status:    model.StatusDeleting,
	}
	remainingKey := model.SSHKey{
		ID:        "other-key",
		TenantID:  tenantID,
		PublicKey: "ssh-rsa AAAAB3... other@test",
		Status:    model.StatusActive,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSSHKeysByTenant", mock.Anything, tenantID).Return([]model.SSHKey{remainingKey}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSSHKeys", mock.Anything, activity.SyncSSHKeysParams{
		TenantName: "t_test123456",
		PublicKeys: []string{"ssh-rsa AAAAB3... other@test"},
	}).Return(nil)

	s.env.ExecuteWorkflow(RemoveSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *RemoveSSHKeyWorkflowTestSuite) TestGetKeyFails_SetsStatusFailed() {
	keyID := "test-key-2"

	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("ssh_keys", keyID)).Return(nil)

	s.env.ExecuteWorkflow(RemoveSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *RemoveSSHKeyWorkflowTestSuite) TestNoShard() {
	keyID := "test-key-3"
	tenantID := "test-tenant-1"
	key := model.SSHKey{
		ID:       keyID,
		TenantID: tenantID,
		Status:   model.StatusDeleting,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		// ShardID is nil
	}

	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)

	s.env.ExecuteWorkflow(RemoveSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *RemoveSSHKeyWorkflowTestSuite) TestSyncFails() {
	keyID := "test-key-4"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"
	key := model.SSHKey{
		ID:       keyID,
		TenantID: tenantID,
		Status:   model.StatusDeleting,
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "t_test123456",
		BrandID: "test-brand",
		ShardID: &shardID,
	}
	nodes := []model.Node{{ID: "node-1"}}

	s.env.OnActivity("GetSSHKeyByID", mock.Anything, keyID).Return(&key, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "ssh_keys", ID: keyID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetSSHKeysByTenant", mock.Anything, tenantID).Return([]model.SSHKey{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("SyncSSHKeys", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))

	s.env.ExecuteWorkflow(RemoveSSHKeyWorkflow, keyID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestAddSSHKeyWorkflow(t *testing.T) {
	suite.Run(t, new(AddSSHKeyWorkflowTestSuite))
}

func TestRemoveSSHKeyWorkflow(t *testing.T) {
	suite.Run(t, new(RemoveSSHKeyWorkflowTestSuite))
}
