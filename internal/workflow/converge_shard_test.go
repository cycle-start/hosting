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

// ---------- ConvergeShardWorkflow ----------

type ConvergeShardWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *ConvergeShardWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	registerActivities(s.env)
}

func (s *ConvergeShardWorkflowTestSuite) AfterTest(suiteName, testName string) {
	s.env.AssertExpectations(s.T())
}

// matchShardStatus returns a matcher for UpdateResourceStatus on the shards table.
func matchShardStatus(shardID, status string) interface{} {
	return mock.MatchedBy(func(params activity.UpdateResourceStatusParams) bool {
		return params.Table == "shards" &&
			params.ID == shardID &&
			params.Status == status
	})
}

func (s *ConvergeShardWorkflowTestSuite) TestWebShard() {
	shardID := "shard-web-1"
	tenantShardID := shardID
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleWeb,
	}
	nodes := []model.Node{
		{ID: "node-1"},
		{ID: "node-2"},
	}
	tenants := []model.Tenant{
		{ID: "tenant-1", BrandID: "test-brand", ShardID: &tenantShardID, UID: 1000, SFTPEnabled: true, Status: model.StatusActive},
	}
	webroots := []model.Webroot{
		{ID: "wr-1", TenantID: "tenant-1", Name: "main", Runtime: "php", RuntimeVersion: "8.5", RuntimeConfig: json.RawMessage(`{}`), PublicFolder: "public", Status: model.StatusActive},
	}
	fqdns := []model.FQDN{
		{ID: "fqdn-1", FQDN: "example.com", WebrootID: "wr-1", SSLEnabled: true, Status: model.StatusActive},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusConverging)).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("ListTenantsByShard", mock.Anything, shardID).Return(tenants, nil)

	// Webroot and FQDN listing (now happens before CleanOrphanedConfigs).
	s.env.OnActivity("ListWebrootsByTenantID", mock.Anything, "tenant-1").Return(webroots, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, "wr-1").Return(fqdns, nil)

	// CleanOrphanedConfigs on each node before creating webroots.
	s.env.OnActivity("CleanOrphanedConfigs", mock.Anything, activity.CleanOrphanedConfigsInput{
		ExpectedConfigs: map[string]bool{"tenant-1_main.conf": true},
	}).Return(activity.CleanOrphanedConfigsResult{}, nil)

	// CreateTenant for each node.
	s.env.OnActivity("CreateTenant", mock.Anything, activity.CreateTenantParams{
		ID: "tenant-1", UID: 1000, SFTPEnabled: true,
	}).Return(nil)
	s.env.OnActivity("SyncSSHConfig", mock.Anything, activity.SyncSSHConfigParams{
		TenantName: "tenant-1", SFTPEnabled: true,
	}).Return(nil)

	// CreateWebroot for each node.
	s.env.OnActivity("CreateWebroot", mock.Anything, activity.CreateWebrootParams{
		ID: "wr-1", TenantName: "tenant-1", Name: "main",
		Runtime: "php", RuntimeVersion: "8.5", RuntimeConfig: "{}",
		PublicFolder: "public",
		FQDNs:        []activity.FQDNParam{{FQDN: "example.com", WebrootID: "wr-1", SSLEnabled: true}},
	}).Return(nil)

	// ListCronJobsByWebroot for each webroot (no cron jobs in this test).
	s.env.OnActivity("ListCronJobsByWebroot", mock.Anything, "wr-1").Return([]model.CronJob{}, nil)

	// ReloadNginx for each node.
	s.env.OnActivity("ReloadNginx", mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusActive)).Return(nil)

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ConvergeShardWorkflowTestSuite) TestDatabaseShard() {
	shardID := "shard-db-1"
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleDatabase,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}
	databases := []model.Database{
		{ID: "db-1", Name: "mydb", ShardID: &shardID, Status: model.StatusActive},
	}
	users := []model.DatabaseUser{
		{ID: "dbu-1", DatabaseID: "db-1", Username: "admin", Password: "pass", Privileges: []string{"ALL"}, Status: model.StatusActive},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusConverging)).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	// SetReadOnly(false) on the primary node.
	s.env.OnActivity("SetReadOnly", mock.Anything, false).Return(nil)
	s.env.OnActivity("ListDatabasesByShard", mock.Anything, shardID).Return(databases, nil)
	s.env.OnActivity("CreateDatabase", mock.Anything, "mydb").Return(nil)
	s.env.OnActivity("ListDatabaseUsersByDatabaseID", mock.Anything, "db-1").Return(users, nil)
	s.env.OnActivity("CreateDatabaseUser", mock.Anything, activity.CreateDatabaseUserParams{
		DatabaseName: "mydb",
		Username:     "admin",
		Password:     "pass",
		Privileges:   []string{"ALL"},
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusActive)).Return(nil)

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ConvergeShardWorkflowTestSuite) TestValkeyShard() {
	shardID := "shard-vk-1"
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleValkey,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}
	instances := []model.ValkeyInstance{
		{ID: "vk-1", Name: "cache", ShardID: &shardID, Port: 6379, Password: "vkpass", MaxMemoryMB: 128, Status: model.StatusActive},
	}
	users := []model.ValkeyUser{
		{ID: "vku-1", ValkeyInstanceID: "vk-1", Username: "app", Password: "apppass", Privileges: []string{"allcommands"}, KeyPattern: "*", Status: model.StatusActive},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusConverging)).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("ListValkeyInstancesByShard", mock.Anything, shardID).Return(instances, nil)
	s.env.OnActivity("CreateValkeyInstance", mock.Anything, activity.CreateValkeyInstanceParams{
		Name:        "cache",
		Port:        6379,
		Password:    "vkpass",
		MaxMemoryMB: 128,
	}).Return(nil)
	s.env.OnActivity("ListValkeyUsersByInstanceID", mock.Anything, "vk-1").Return(users, nil)
	s.env.OnActivity("CreateValkeyUser", mock.Anything, activity.CreateValkeyUserParams{
		InstanceName: "cache",
		Port:         6379,
		Username:     "app",
		Password:     "apppass",
		Privileges:   []string{"allcommands"},
		KeyPattern:   "*",
	}).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusActive)).Return(nil)

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ConvergeShardWorkflowTestSuite) TestNoNodes() {
	shardID := "shard-empty"
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleWeb,
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusConverging)).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return([]model.Node{}, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusFailed)).Return(nil)

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ConvergeShardWorkflowTestSuite) TestSkipsInactiveResources() {
	shardID := "shard-web-2"
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleWeb,
	}
	nodes := []model.Node{
		{ID: "node-1"},
	}
	// A provisioning tenant should be skipped.
	tenants := []model.Tenant{
		{ID: "tenant-active", BrandID: "test-brand", UID: 1000, Status: model.StatusActive},
		{ID: "tenant-prov", BrandID: "test-brand", UID: 1001, Status: model.StatusProvisioning},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusConverging)).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("ListTenantsByShard", mock.Anything, shardID).Return(tenants, nil)

	// Webroot listing for active tenant (no webroots).
	s.env.OnActivity("ListWebrootsByTenantID", mock.Anything, "tenant-active").Return([]model.Webroot{}, nil)

	// CleanOrphanedConfigs with empty expected set (no active webroots).
	s.env.OnActivity("CleanOrphanedConfigs", mock.Anything, activity.CleanOrphanedConfigsInput{
		ExpectedConfigs: map[string]bool{},
	}).Return(activity.CleanOrphanedConfigsResult{}, nil)

	// Only the active tenant gets CreateTenant.
	s.env.OnActivity("CreateTenant", mock.Anything, activity.CreateTenantParams{
		ID: "tenant-active", UID: 1000,
	}).Return(nil)
	s.env.OnActivity("SyncSSHConfig", mock.Anything, activity.SyncSSHConfigParams{
		TenantName: "tenant-active",
	}).Return(nil)

	s.env.OnActivity("ReloadNginx", mock.Anything).Return(nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusActive)).Return(nil)

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ConvergeShardWorkflowTestSuite) TestGetShardFails() {
	shardID := "shard-fail"
	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(nil, fmt.Errorf("not found"))

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
}

func (s *ConvergeShardWorkflowTestSuite) TestPartialFailureContinues() {
	shardID := "shard-db-partial"
	primaryIP := "10.0.0.1"
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleDatabase,
	}
	nodes := []model.Node{
		{ID: "node-ok", IPAddress: &primaryIP},
		{ID: "node-bad"},
	}
	databases := []model.Database{
		{ID: "db-1", Name: "mydb", ShardID: &shardID, Status: model.StatusActive},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusConverging)).Return(nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)

	// SetReadOnly(false) on primary (node-ok).
	s.env.OnActivity("SetReadOnly", mock.Anything, false).Return(nil)

	s.env.OnActivity("ListDatabasesByShard", mock.Anything, shardID).Return(databases, nil)

	// CreateDatabase on primary only - succeeds.
	s.env.OnActivity("CreateDatabase", mock.Anything, "mydb").Return(nil)

	// User listing still runs (workflow doesn't stop).
	s.env.OnActivity("ListDatabaseUsersByDatabaseID", mock.Anything, "db-1").Return([]model.DatabaseUser{}, nil)

	// SetReadOnly(true) on replica (node-bad) - fails.
	s.env.OnActivity("SetReadOnly", mock.Anything, true).Return(fmt.Errorf("node unreachable"))

	// GetReplicationStatus on replica - fails.
	s.env.OnActivity("GetReplicationStatus", mock.Anything).Return(nil, fmt.Errorf("node unreachable"))

	// ConfigureReplication on replica - fails.
	s.env.OnActivity("ConfigureReplication", mock.Anything, activity.ConfigureReplicationParams{
		PrimaryHost: primaryIP, ReplUser: "repl", ReplPassword: "repl",
	}).Return(fmt.Errorf("node unreachable"))

	// Should end in failed state with error message.
	s.env.OnActivity("UpdateResourceStatus", mock.Anything, matchShardStatus(shardID, model.StatusFailed)).Return(nil)

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.Error(s.env.GetWorkflowError())
	s.Contains(s.env.GetWorkflowError().Error(), "convergence completed with")
}

// ---------- Run ----------

func TestConvergeShardWorkflow(t *testing.T) {
	suite.Run(t, new(ConvergeShardWorkflowTestSuite))
}
