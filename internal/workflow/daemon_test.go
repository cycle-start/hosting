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

// ---------- CreateDaemonWorkflow ----------

type CreateDaemonWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *CreateDaemonWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *CreateDaemonWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *CreateDaemonWorkflowTestSuite) TestSuccess_NoProxy() {
	daemonID := "daemon-1"
	shardID := "shard-1"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon: model.Daemon{
			ID:           daemonID,
			TenantID:     "tenant-1",
			NodeID:       &nodeID,
			WebrootID:    "wr-1",
			Name:         "daemon_abc123",
			Command:      "php artisan queue:work",
			NumProcs:     2,
			StopSignal:   "TERM",
			StopWaitSecs: 30,
			MaxMemoryMB:  256,
			Environment:  map[string]string{},
		},
		Webroot: model.Webroot{
			ID:             "wr-1",
			TenantID:       "tenant-1",
			Name:           "main",
			Runtime:        "php",
			RuntimeVersion: "8.5",
			RuntimeConfig:  json.RawMessage(`{}`),
			PublicFolder:   "public",
		},
		Tenant: model.Tenant{
			ID:      "tenant-1",
			Name:    "t_test123456",
			BrandID: "test-brand",
			ShardID: &shardID,
		},
		Nodes: []model.Node{{ID: "node-1"}, {ID: "node-2"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)

	// CreateDaemonConfig on the daemon's assigned node only.
	s.env.OnActivity("CreateDaemonConfig", mock.Anything, activity.CreateDaemonParams{
		ID:           daemonID,
		NodeID:       &nodeID,
		TenantName:   "t_test123456",
		WebrootName:  "main",
		Name:         "daemon_abc123",
		Command:      "php artisan queue:work",
		NumProcs:     2,
		StopSignal:   "TERM",
		StopWaitSecs: 30,
		MaxMemoryMB:  256,
		Environment:  map[string]string{},
	}).Return(nil)

	// No proxy_path, so no nginx regeneration.

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateDaemonWorkflowTestSuite) TestSuccess_WithProxy() {
	daemonID := "daemon-2"
	shardID := "shard-1"
	nodeID := "node-1"
	proxyPath := "/app"
	proxyPort := 14523
	daemonCtx := activity.DaemonContext{
		Daemon: model.Daemon{
			ID:           daemonID,
			TenantID:     "tenant-1",
			NodeID:       &nodeID,
			WebrootID:    "wr-1",
			Name:         "daemon_xyz789",
			Command:      "php artisan reverb:start --port=$PORT",
			ProxyPath:    &proxyPath,
			ProxyPort:    &proxyPort,
			NumProcs:     1,
			StopSignal:   "TERM",
			StopWaitSecs: 30,
			MaxMemoryMB:  256,
			Environment:  map[string]string{"APP_ENV": "production"},
		},
		Webroot: model.Webroot{
			ID:             "wr-1",
			TenantID:       "tenant-1",
			Name:           "main",
			Runtime:        "php",
			RuntimeVersion: "8.5",
			RuntimeConfig:  json.RawMessage(`{}`),
			PublicFolder:   "public",
		},
		Tenant: model.Tenant{
			ID:      "tenant-1",
			Name:    "t_test123456",
			BrandID: "test-brand",
			ShardID: &shardID,
		},
		Nodes: []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)

	s.env.OnActivity("CreateDaemonConfig", mock.Anything, activity.CreateDaemonParams{
		ID:           daemonID,
		NodeID:       &nodeID,
		TenantName:   "t_test123456",
		WebrootName:  "main",
		Name:         "daemon_xyz789",
		Command:      "php artisan reverb:start --port=$PORT",
		ProxyPort:    &proxyPort,
		NumProcs:     1,
		StopSignal:   "TERM",
		StopWaitSecs: 30,
		MaxMemoryMB:  256,
		Environment:  map[string]string{"APP_ENV": "production"},
	}).Return(nil)

	// With proxy_path: regenerate nginx on all nodes (ListDaemonsByWebroot + GetFQDNsByWebrootID + UpdateWebroot + ReloadNginx).
	s.env.OnActivity("ListDaemonsByWebroot", mock.Anything, "wr-1").Return([]model.Daemon{daemonCtx.Daemon}, nil)
	wr1 := "wr-1"
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, "wr-1").Return([]model.FQDN{
		{FQDN: "example.com", WebrootID: &wr1, SSLEnabled: true, Status: model.StatusActive},
	}, nil)
	s.env.OnActivity("UpdateWebroot", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("ReloadNginx", mock.Anything).Return(nil)

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(CreateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *CreateDaemonWorkflowTestSuite) TestGetContextFails() {
	daemonID := "daemon-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(CreateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateDaemonWorkflowTestSuite) TestNoShard() {
	daemonID := "daemon-4"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID},
		Webroot: model.Webroot{ID: "wr-1"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand"},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(CreateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateDaemonWorkflowTestSuite) TestNoNode() {
	daemonID := "daemon-6"
	shardID := "shard-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1"},
		Webroot: model.Webroot{ID: "wr-1"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(CreateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *CreateDaemonWorkflowTestSuite) TestSetProvisioningFails() {
	daemonID := "daemon-5"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(fmt.Errorf("db error"))

	s.env.ExecuteWorkflow(CreateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DeleteDaemonWorkflow ----------

type DeleteDaemonWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DeleteDaemonWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DeleteDaemonWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DeleteDaemonWorkflowTestSuite) TestSuccess_NoProxy() {
	daemonID := "daemon-1"
	shardID := "shard-1"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID, WebrootID: "wr-1", Name: "daemon_abc123", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("DeleteDaemonConfig", mock.Anything, activity.DeleteDaemonParams{
		ID: daemonID, TenantName: "t_test123456", WebrootName: "main", Name: "daemon_abc123",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusDeleted,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteDaemonWorkflowTestSuite) TestSuccess_WithProxy() {
	daemonID := "daemon-2"
	shardID := "shard-1"
	nodeID := "node-1"
	proxyPath := "/ws"
	proxyPort := 15000
	daemonCtx := activity.DaemonContext{
		Daemon: model.Daemon{
			ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID, WebrootID: "wr-1", Name: "daemon_ws123",
			ProxyPath: &proxyPath, ProxyPort: &proxyPort, Environment: map[string]string{},
		},
		Webroot: model.Webroot{ID: "wr-1", Name: "main", Runtime: "php", RuntimeVersion: "8.5", RuntimeConfig: json.RawMessage(`{}`), PublicFolder: "public"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("DeleteDaemonConfig", mock.Anything, activity.DeleteDaemonParams{
		ID: daemonID, TenantName: "t_test123456", WebrootName: "main", Name: "daemon_ws123",
	}).Return(nil)

	// With proxy_path: regenerate nginx on all nodes to remove proxy location.
	s.env.OnActivity("ListDaemonsByWebroot", mock.Anything, "wr-1").Return([]model.Daemon{}, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, "wr-1").Return([]model.FQDN{}, nil)
	s.env.OnActivity("UpdateWebroot", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("ReloadNginx", mock.Anything).Return(nil)

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusDeleted,
	}).Return(nil)

	s.env.ExecuteWorkflow(DeleteDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DeleteDaemonWorkflowTestSuite) TestGetContextFails() {
	daemonID := "daemon-3"

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(nil, fmt.Errorf("not found"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(DeleteDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DeleteDaemonWorkflowTestSuite) TestNoNode() {
	daemonID := "daemon-4"
	shardID := "shard-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", WebrootID: "wr-1", Name: "daemon_abc123", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusDeleting,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(DeleteDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- EnableDaemonWorkflow ----------

type EnableDaemonWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *EnableDaemonWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *EnableDaemonWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *EnableDaemonWorkflowTestSuite) TestSuccess() {
	daemonID := "daemon-1"
	shardID := "shard-1"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID, WebrootID: "wr-1", Name: "daemon_abc", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}, {ID: "node-2"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("EnableDaemon", mock.Anything, activity.DaemonEnableParams{
		ID: daemonID, TenantName: "t_test123456", WebrootName: "main", Name: "daemon_abc",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(EnableDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *EnableDaemonWorkflowTestSuite) TestNodeFails() {
	daemonID := "daemon-2"
	shardID := "shard-1"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID, WebrootID: "wr-1", Name: "daemon_abc", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("EnableDaemon", mock.Anything, mock.Anything).Return(fmt.Errorf("node down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(EnableDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *EnableDaemonWorkflowTestSuite) TestNoNode() {
	daemonID := "daemon-3"
	shardID := "shard-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", WebrootID: "wr-1", Name: "daemon_abc", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(EnableDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- DisableDaemonWorkflow ----------

type DisableDaemonWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *DisableDaemonWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *DisableDaemonWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *DisableDaemonWorkflowTestSuite) TestSuccess() {
	daemonID := "daemon-1"
	shardID := "shard-1"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID, WebrootID: "wr-1", Name: "daemon_abc", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("DisableDaemon", mock.Anything, activity.DaemonEnableParams{
		ID: daemonID, TenantName: "t_test123456", WebrootName: "main", Name: "daemon_abc",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(DisableDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *DisableDaemonWorkflowTestSuite) TestNodeFails() {
	daemonID := "daemon-2"
	shardID := "shard-1"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID, WebrootID: "wr-1", Name: "daemon_abc", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("DisableDaemon", mock.Anything, mock.Anything).Return(fmt.Errorf("node down"))
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(DisableDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *DisableDaemonWorkflowTestSuite) TestNoNode() {
	daemonID := "daemon-3"
	shardID := "shard-1"
	daemonCtx := activity.DaemonContext{
		Daemon:  model.Daemon{ID: daemonID, TenantID: "tenant-1", WebrootID: "wr-1", Name: "daemon_abc", Environment: map[string]string{}},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(DisableDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- UpdateDaemonWorkflow ----------

type UpdateDaemonWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateDaemonWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *UpdateDaemonWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

func (s *UpdateDaemonWorkflowTestSuite) TestSuccess() {
	daemonID := "daemon-1"
	shardID := "shard-1"
	nodeID := "node-1"
	daemonCtx := activity.DaemonContext{
		Daemon: model.Daemon{
			ID: daemonID, TenantID: "tenant-1", NodeID: &nodeID, WebrootID: "wr-1", Name: "daemon_abc",
			Command: "node server.js", NumProcs: 1, StopSignal: "TERM",
			StopWaitSecs: 30, MaxMemoryMB: 256, Environment: map[string]string{},
		},
		Webroot: model.Webroot{ID: "wr-1", Name: "main", Runtime: "node", RuntimeVersion: "22", RuntimeConfig: json.RawMessage(`{}`), PublicFolder: ""},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("UpdateDaemonConfig", mock.Anything, mock.MatchedBy(func(p activity.CreateDaemonParams) bool {
		return p.ID == daemonID && p.Command == "node server.js"
	})).Return(nil)

	// UpdateDaemonWorkflow always regenerates nginx on all nodes.
	s.env.OnActivity("ListDaemonsByWebroot", mock.Anything, "wr-1").Return([]model.Daemon{}, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, "wr-1").Return([]model.FQDN{}, nil)
	s.env.OnActivity("UpdateWebroot", mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity("ReloadNginx", mock.Anything).Return(nil)

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusActive,
	}).Return(nil)

	s.env.ExecuteWorkflow(UpdateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *UpdateDaemonWorkflowTestSuite) TestNoNode() {
	daemonID := "daemon-2"
	shardID := "shard-1"
	daemonCtx := activity.DaemonContext{
		Daemon: model.Daemon{
			ID: daemonID, TenantID: "tenant-1", WebrootID: "wr-1", Name: "daemon_abc",
			Command: "node server.js", NumProcs: 1, StopSignal: "TERM",
			StopWaitSecs: 30, MaxMemoryMB: 256, Environment: map[string]string{},
		},
		Webroot: model.Webroot{ID: "wr-1", Name: "main"},
		Tenant:  model.Tenant{ID: "tenant-1", Name: "t_test123456", BrandID: "test-brand", ShardID: &shardID},
		Nodes:   []model.Node{{ID: "node-1"}},
	}

	s.env.OnActivity("UpdateResourceStatus", mock.Anything, activity.UpdateResourceStatusParams{
		Table: "daemons", ID: daemonID, Status: model.StatusProvisioning,
	}).Return(nil)
	s.env.OnActivity("GetDaemonContext", mock.Anything, daemonID).Return(&daemonCtx, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchFailedStatus("daemons", daemonID)).Return(nil)

	s.env.ExecuteWorkflow(UpdateDaemonWorkflow, daemonID)
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

// ---------- Run ----------

func TestCreateDaemonWorkflow(t *testing.T) {
	suite.Run(t, new(CreateDaemonWorkflowTestSuite))
}

func TestDeleteDaemonWorkflow(t *testing.T) {
	suite.Run(t, new(DeleteDaemonWorkflowTestSuite))
}

func TestEnableDaemonWorkflow(t *testing.T) {
	suite.Run(t, new(EnableDaemonWorkflowTestSuite))
}

func TestDisableDaemonWorkflow(t *testing.T) {
	suite.Run(t, new(DisableDaemonWorkflowTestSuite))
}

func TestUpdateDaemonWorkflow(t *testing.T) {
	suite.Run(t, new(UpdateDaemonWorkflowTestSuite))
}
