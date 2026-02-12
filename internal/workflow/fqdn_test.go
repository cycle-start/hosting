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

// ---------- BindFQDNWorkflow ----------

type BindFQDNWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *BindFQDNWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *BindFQDNWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *BindFQDNWorkflowTestSuite) TestSuccess_NoSSL() {
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"
	shardID := "test-shard-1"

	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "example.com",
		WebrootID:  webrootID,
		SSLEnabled: false,
	}
	webroot := model.Webroot{
		ID:       webrootID,
		TenantID: tenantID,
	}
	tenant := model.Tenant{
		ID:        tenantID,
		Name:      "testuser",
		ClusterID: clusterID,
		ShardID:   &shardID,
	}
	lbAddresses := []model.ClusterLBAddress{
		{ID: "test-lb-1", ClusterID: clusterID, Address: "10.0.0.1", Family: 4, Label: "primary"},
		{ID: "test-lb-2", ClusterID: clusterID, Address: "::1", Family: 6, Label: "primary"},
	}
	shard := model.Shard{
		ID:        shardID,
		LBBackend: "backend-1.example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterLBAddresses", mock.Anything, clusterID).Return(lbAddresses, nil)
	s.env.OnActivity("AutoCreateDNSRecords", mock.Anything, activity.AutoCreateDNSRecordsParams{
		FQDN:         "example.com",
		LBAddresses:  lbAddresses,
		SourceFQDNID: fqdnID,
	}).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, mock.Anything).Return([]model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(nil)
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("SetLBMapEntry", mock.Anything, activity.SetLBMapEntryParams{
		ClusterID: clusterID,
		FQDN:      "example.com",
		LBBackend: "backend-1.example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(BindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *BindFQDNWorkflowTestSuite) TestSuccess_WithSSL() {
	fqdnID := "test-fqdn-2"
	webrootID := "test-webroot-2"
	tenantID := "test-tenant-2"
	clusterID := "test-cluster-2"
	shardID := "test-shard-2"

	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "secure.example.com",
		WebrootID:  webrootID,
		SSLEnabled: true,
	}
	webroot := model.Webroot{
		ID:       webrootID,
		TenantID: tenantID,
	}
	tenant := model.Tenant{
		ID:        tenantID,
		Name:      "testuser",
		ClusterID: clusterID,
		ShardID:   &shardID,
	}
	lbAddresses := []model.ClusterLBAddress{
		{ID: "test-lb-1", ClusterID: clusterID, Address: "10.0.0.1", Family: 4, Label: "primary"},
		{ID: "test-lb-2", ClusterID: clusterID, Address: "::1", Family: 6, Label: "primary"},
	}
	shard := model.Shard{
		ID:        shardID,
		LBBackend: "backend-1.example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterLBAddresses", mock.Anything, clusterID).Return(lbAddresses, nil)
	s.env.OnActivity("AutoCreateDNSRecords", mock.Anything, activity.AutoCreateDNSRecordsParams{
		FQDN:         "secure.example.com",
		LBAddresses:  lbAddresses,
		SourceFQDNID: fqdnID,
	}).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, mock.Anything).Return([]model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(nil)
	// The child workflow for LE cert provisioning will be registered separately.
	// Mock the child workflow to succeed.
	s.env.OnWorkflow(ProvisionLECertWorkflow, mock.Anything, fqdnID).Return(nil)
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("SetLBMapEntry", mock.Anything, activity.SetLBMapEntryParams{
		ClusterID: clusterID,
		FQDN:      "secure.example.com",
		LBBackend: "backend-1.example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(BindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *BindFQDNWorkflowTestSuite) TestSuccess_SSLChildWorkflowFails_StillSucceeds() {
	fqdnID := "test-fqdn-3"
	webrootID := "test-webroot-3"
	tenantID := "test-tenant-3"
	clusterID := "test-cluster-3"
	shardID := "test-shard-3"

	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "secure.example.com",
		WebrootID:  webrootID,
		SSLEnabled: true,
	}
	webroot := model.Webroot{
		ID:       webrootID,
		TenantID: tenantID,
	}
	tenant := model.Tenant{
		ID:        tenantID,
		Name:      "testuser",
		ClusterID: clusterID,
		ShardID:   &shardID,
	}
	lbAddresses := []model.ClusterLBAddress{
		{ID: "test-lb-1", ClusterID: clusterID, Address: "10.0.0.1", Family: 4, Label: "primary"},
		{ID: "test-lb-2", ClusterID: clusterID, Address: "::1", Family: 6, Label: "primary"},
	}
	shard := model.Shard{
		ID:        shardID,
		LBBackend: "backend-1.example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterLBAddresses", mock.Anything, clusterID).Return(lbAddresses, nil)
	s.env.OnActivity("AutoCreateDNSRecords", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, mock.Anything).Return([]model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(nil)
	// Child LE workflow fails -- but FQDN binding should still succeed.
	s.env.OnWorkflow(ProvisionLECertWorkflow, mock.Anything, fqdnID).Return(fmt.Errorf("ACME failed"))
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("SetLBMapEntry", mock.Anything, activity.SetLBMapEntryParams{
		ClusterID: clusterID,
		FQDN:      "secure.example.com",
		LBBackend: "backend-1.example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(BindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *BindFQDNWorkflowTestSuite) TestAutoCreateDNSFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-4"
	webrootID := "test-webroot-4"
	tenantID := "test-tenant-4"
	clusterID := "test-cluster-4"

	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "example.com",
		WebrootID:  webrootID,
		SSLEnabled: false,
	}
	webroot := model.Webroot{
		ID:       webrootID,
		TenantID: tenantID,
	}
	tenant := model.Tenant{
		ID:        tenantID,
		Name:      "testuser",
		ClusterID: clusterID,
	}
	lbAddresses := []model.ClusterLBAddress{
		{ID: "test-lb-1", ClusterID: clusterID, Address: "10.0.0.1", Family: 4, Label: "primary"},
		{ID: "test-lb-2", ClusterID: clusterID, Address: "::1", Family: 6, Label: "primary"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterLBAddresses", mock.Anything, clusterID).Return(lbAddresses, nil)
	s.env.OnActivity("AutoCreateDNSRecords", mock.Anything, mock.Anything).Return(fmt.Errorf("dns error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(BindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *BindFQDNWorkflowTestSuite) TestGetFQDNFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-5"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(BindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *BindFQDNWorkflowTestSuite) TestSetLBMapEntryFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-6"
	webrootID := "test-webroot-6"
	tenantID := "test-tenant-6"
	clusterID := "test-cluster-6"
	shardID := "test-shard-6"

	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "example.com",
		WebrootID:  webrootID,
		SSLEnabled: false,
	}
	webroot := model.Webroot{
		ID:       webrootID,
		TenantID: tenantID,
	}
	tenant := model.Tenant{
		ID:        tenantID,
		Name:      "testuser",
		ClusterID: clusterID,
		ShardID:   &shardID,
	}
	lbAddresses := []model.ClusterLBAddress{
		{ID: "test-lb-1", ClusterID: clusterID, Address: "10.0.0.1", Family: 4, Label: "primary"},
		{ID: "test-lb-2", ClusterID: clusterID, Address: "::1", Family: 6, Label: "primary"},
	}
	shard := model.Shard{
		ID:        shardID,
		LBBackend: "backend-1.example.com",
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterLBAddresses", mock.Anything, clusterID).Return(lbAddresses, nil)
	s.env.OnActivity("AutoCreateDNSRecords", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, mock.Anything).Return([]model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(nil)
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("SetLBMapEntry", mock.Anything, activity.SetLBMapEntryParams{
		ClusterID: clusterID,
		FQDN:      "example.com",
		LBBackend: "backend-1.example.com",
	}).Return(fmt.Errorf("lb map error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(BindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *BindFQDNWorkflowTestSuite) TestReloadNginxFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-7"
	webrootID := "test-webroot-7"
	tenantID := "test-tenant-7"
	clusterID := "test-cluster-7"
	shardID := "test-shard-7"

	fqdn := model.FQDN{
		ID:         fqdnID,
		FQDN:       "example.com",
		WebrootID:  webrootID,
		SSLEnabled: false,
	}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, Name: "testuser", ClusterID: clusterID, ShardID: &shardID}
	lbAddresses := []model.ClusterLBAddress{
		{ID: "test-lb-1", ClusterID: clusterID, Address: "10.0.0.1", Family: 4, Label: "primary"},
		{ID: "test-lb-2", ClusterID: clusterID, Address: "::1", Family: 6, Label: "primary"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetClusterLBAddresses", mock.Anything, clusterID).Return(lbAddresses, nil)
	s.env.OnActivity("AutoCreateDNSRecords", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return([]model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(fmt.Errorf("nginx error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(BindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UnbindFQDNWorkflow ----------

type UnbindFQDNWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UnbindFQDNWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UnbindFQDNWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UnbindFQDNWorkflowTestSuite) TestSuccess() {
	fqdnID := "test-fqdn-1"
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	clusterID := "test-cluster-1"
	shardID := "test-shard-1"

	fqdn := model.FQDN{
		ID:        fqdnID,
		FQDN:      "example.com",
		WebrootID: webrootID,
	}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, Name: "testuser", ClusterID: clusterID, ShardID: &shardID}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("AutoDeleteDNSRecords", mock.Anything, "example.com").Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return([]model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(nil)
	s.env.OnActivity("DeleteLBMapEntry", mock.Anything, activity.DeleteLBMapEntryParams{
		ClusterID: clusterID,
		FQDN:      "example.com",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(UnbindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UnbindFQDNWorkflowTestSuite) TestAutoDeleteDNSFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-2"
	webrootID := "test-webroot-2"
	tenantID := "test-tenant-2"

	fqdn := model.FQDN{
		ID:        fqdnID,
		FQDN:      "example.com",
		WebrootID: webrootID,
	}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, Name: "testuser"}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("AutoDeleteDNSRecords", mock.Anything, "example.com").Return(fmt.Errorf("dns error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UnbindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UnbindFQDNWorkflowTestSuite) TestGetFQDNFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UnbindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *UnbindFQDNWorkflowTestSuite) TestReloadNginxFails_SetsStatusFailed() {
	fqdnID := "test-fqdn-4"
	webrootID := "test-webroot-4"
	tenantID := "test-tenant-4"
	shardID := "test-shard-4"

	fqdn := model.FQDN{
		ID:        fqdnID,
		FQDN:      "example.com",
		WebrootID: webrootID,
	}
	webroot := model.Webroot{ID: webrootID, TenantID: tenantID}
	tenant := model.Tenant{ID: tenantID, Name: "testuser", ClusterID: "test-cluster-4", ShardID: &shardID}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetFQDNByID", mock.Anything, fqdnID).Return(&fqdn, nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("AutoDeleteDNSRecords", mock.Anything, "example.com").Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return([]model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(fmt.Errorf("nginx error"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "fqdns", ID: fqdnID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UnbindFQDNWorkflow, fqdnID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestBindFQDNWorkflow(t *testing.T) {
	suite.Run(t, new(BindFQDNWorkflowTestSuite))
}

func TestUnbindFQDNWorkflow(t *testing.T) {
	suite.Run(t, new(UnbindFQDNWorkflowTestSuite))
}
