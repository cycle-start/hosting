package workflow

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// ---------- CreateWebrootWorkflow ----------

type CreateWebrootWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateWebrootWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateWebrootWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateWebrootWorkflowTestSuite) TestSuccess() {
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	fqdnID := "test-fqdn-1"
	shardID := "test-shard-1"

	webroot := model.Webroot{
		ID:             webrootID,
		TenantID:       tenantID,
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "8.2",
		RuntimeConfig:  json.RawMessage(`{}`),
		PublicFolder:   "public",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "testuser",
		UID:     5001,
		ShardID: &shardID,
	}
	fqdns := []model.FQDN{
		{ID: fqdnID, FQDN: "example.com", WebrootID: webrootID, SSLEnabled: true},
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, webrootID).Return(fqdns, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateWebroot", mock.Anything, activity.CreateWebrootParams{
		ID:             webrootID,
		TenantName:     "testuser",
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "8.2",
		RuntimeConfig:  "{}",
		PublicFolder:   "public",
		FQDNs: []activity.FQDNParam{
			{FQDN: "example.com", WebrootID: webrootID, SSLEnabled: true},
		},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateWebrootWorkflowTestSuite) TestNoFQDNs_Success() {
	webrootID := "test-webroot-2"
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"

	webroot := model.Webroot{
		ID:             webrootID,
		TenantID:       tenantID,
		Name:           "mysite",
		Runtime:        "static",
		RuntimeVersion: "",
		RuntimeConfig:  json.RawMessage(`{}`),
		PublicFolder:   ".",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "testuser",
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, webrootID).Return([]model.FQDN{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateWebroot", mock.Anything, activity.CreateWebrootParams{
		ID:             webrootID,
		TenantName:     "testuser",
		Name:           "mysite",
		Runtime:        "static",
		RuntimeVersion: "",
		RuntimeConfig:  "{}",
		PublicFolder:   ".",
		FQDNs:          []activity.FQDNParam{},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateWebrootWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	webrootID := "test-webroot-3"
	tenantID := "test-tenant-3"
	shardID := "test-shard-3"

	webroot := model.Webroot{
		ID:             webrootID,
		TenantID:       tenantID,
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "8.2",
		RuntimeConfig:  json.RawMessage(`{}`),
		PublicFolder:   "public",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "testuser",
		UID:     5001,
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, webrootID).Return([]model.FQDN{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("CreateWebroot", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateWebrootWorkflowTestSuite) TestGetWebrootFails_SetsStatusFailed() {
	webrootID := "test-webroot-4"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(CreateWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UpdateWebrootWorkflow ----------

type UpdateWebrootWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateWebrootWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UpdateWebrootWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateWebrootWorkflowTestSuite) TestSuccess() {
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"

	webroot := model.Webroot{
		ID:             webrootID,
		TenantID:       tenantID,
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "8.3",
		RuntimeConfig:  json.RawMessage(`{"memory_limit":"256M"}`),
		PublicFolder:   "public",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "testuser",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, webrootID).Return([]model.FQDN{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("UpdateWebroot", mock.Anything, activity.UpdateWebrootParams{
		ID:             webrootID,
		TenantName:     "testuser",
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "8.3",
		RuntimeConfig:  `{"memory_limit":"256M"}`,
		PublicFolder:   "public",
		FQDNs:          []activity.FQDNParam{},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusActive,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateWebrootWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	webrootID := "test-webroot-2"
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"

	webroot := model.Webroot{
		ID:             webrootID,
		TenantID:       tenantID,
		Name:           "mysite",
		Runtime:        "php",
		RuntimeVersion: "8.3",
		RuntimeConfig:  json.RawMessage(`{}`),
		PublicFolder:   "public",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "testuser",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, webrootID).Return([]model.FQDN{}, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("UpdateWebroot", mock.Anything, mock.Anything).Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(UpdateWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteWebrootWorkflow ----------

type DeleteWebrootWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteWebrootWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteWebrootWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteWebrootWorkflowTestSuite) TestSuccess() {
	webrootID := "test-webroot-1"
	tenantID := "test-tenant-1"
	shardID := "test-shard-1"

	webroot := model.Webroot{
		ID:       webrootID,
		TenantID: tenantID,
		Name:     "mysite",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "testuser",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteWebroot", mock.Anything, "testuser", "mysite").Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusDeleted,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteWebrootWorkflowTestSuite) TestAgentFails_SetsStatusFailed() {
	webrootID := "test-webroot-2"
	tenantID := "test-tenant-2"
	shardID := "test-shard-2"

	webroot := model.Webroot{
		ID:       webrootID,
		TenantID: tenantID,
		Name:     "mysite",
	}
	tenant := model.Tenant{
		ID:      tenantID,
		Name:    "testuser",
		ShardID: &shardID,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(&webroot, nil)
	s.env.OnActivity("GetTenantByID", mock.Anything, tenantID).Return(&tenant, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("DeleteWebroot", mock.Anything, "testuser", "mysite").Return(fmt.Errorf("node agent down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteWebrootWorkflowTestSuite) TestGetWebrootFails_SetsStatusFailed() {
	webrootID := "test-webroot-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetWebrootByID", mock.Anything, webrootID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "webroots", ID: webrootID, Status: model.StatusFailed,
	}).Return(nil)
	s.env.ExecuteWorkflow(DeleteWebrootWorkflow, webrootID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run all suites ----------

func TestCreateWebrootWorkflow(t *testing.T) {
	suite.Run(t, new(CreateWebrootWorkflowTestSuite))
}

func TestUpdateWebrootWorkflow(t *testing.T) {
	suite.Run(t, new(UpdateWebrootWorkflowTestSuite))
}

func TestDeleteWebrootWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteWebrootWorkflowTestSuite))
}
