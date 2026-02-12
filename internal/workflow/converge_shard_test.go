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

func (s *ConvergeShardWorkflowTestSuite) TestWebShard() {
	shardID := "shard-web-1"
	tenantShardID := shardID
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleWeb,
	}
	nodes := []model.Node{
		{ID: "node-1", GRPCAddress: "node1:9090"},
		{ID: "node-2", GRPCAddress: "node2:9090"},
	}
	tenants := []model.Tenant{
		{ID: "tenant-1", Name: "example", ShardID: &tenantShardID, UID: 1000, SFTPEnabled: true, Status: model.StatusActive},
	}
	webroots := []model.Webroot{
		{ID: "wr-1", TenantID: "tenant-1", Name: "main", Runtime: "php", RuntimeVersion: "8.5", RuntimeConfig: json.RawMessage(`{}`), PublicFolder: "public", Status: model.StatusActive},
	}
	fqdns := []model.FQDN{
		{ID: "fqdn-1", FQDN: "example.com", WebrootID: "wr-1", SSLEnabled: true, Status: model.StatusActive},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("ListTenantsByShard", mock.Anything, shardID).Return(tenants, nil)

	// CreateTenantOnNode for each node
	s.env.OnActivity("CreateTenantOnNode", mock.Anything, activity.CreateTenantOnNodeParams{
		NodeAddress: "node1:9090",
		Tenant:      activity.CreateTenantParams{ID: "tenant-1", Name: "example", UID: 1000, SFTPEnabled: true},
	}).Return(nil)
	s.env.OnActivity("CreateTenantOnNode", mock.Anything, activity.CreateTenantOnNodeParams{
		NodeAddress: "node2:9090",
		Tenant:      activity.CreateTenantParams{ID: "tenant-1", Name: "example", UID: 1000, SFTPEnabled: true},
	}).Return(nil)

	s.env.OnActivity("ListWebrootsByTenantID", mock.Anything, "tenant-1").Return(webroots, nil)
	s.env.OnActivity("GetFQDNsByWebrootID", mock.Anything, "wr-1").Return(fqdns, nil)

	// CreateWebrootOnNode for each node
	s.env.OnActivity("CreateWebrootOnNode", mock.Anything, activity.CreateWebrootOnNodeParams{
		NodeAddress: "node1:9090",
		Webroot: activity.CreateWebrootParams{
			ID: "wr-1", TenantName: "example", Name: "main",
			Runtime: "php", RuntimeVersion: "8.5", RuntimeConfig: "{}",
			PublicFolder: "public",
			FQDNs:        []activity.FQDNParam{{FQDN: "example.com", WebrootID: "wr-1", SSLEnabled: true}},
		},
	}).Return(nil)
	s.env.OnActivity("CreateWebrootOnNode", mock.Anything, activity.CreateWebrootOnNodeParams{
		NodeAddress: "node2:9090",
		Webroot: activity.CreateWebrootParams{
			ID: "wr-1", TenantName: "example", Name: "main",
			Runtime: "php", RuntimeVersion: "8.5", RuntimeConfig: "{}",
			PublicFolder: "public",
			FQDNs:        []activity.FQDNParam{{FQDN: "example.com", WebrootID: "wr-1", SSLEnabled: true}},
		},
	}).Return(nil)

	// ReloadNginxOnNode for each node
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node2:9090").Return(nil)

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
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}
	databases := []model.Database{
		{ID: "db-1", Name: "mydb", ShardID: &shardID, Status: model.StatusActive},
	}
	users := []model.DatabaseUser{
		{ID: "dbu-1", DatabaseID: "db-1", Username: "admin", Password: "pass", Privileges: []string{"ALL"}, Status: model.StatusActive},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("ListDatabasesByShard", mock.Anything, shardID).Return(databases, nil)
	s.env.OnActivity("CreateDatabaseOnNode", mock.Anything, activity.CreateDatabaseOnNodeParams{
		NodeAddress: "node1:9090",
		Name:        "mydb",
	}).Return(nil)
	s.env.OnActivity("ListDatabaseUsersByDatabaseID", mock.Anything, "db-1").Return(users, nil)
	s.env.OnActivity("CreateDatabaseUserOnNode", mock.Anything, activity.CreateDatabaseUserOnNodeParams{
		NodeAddress: "node1:9090",
		User: activity.CreateDatabaseUserParams{
			DatabaseName: "mydb",
			Username:     "admin",
			Password:     "pass",
			Privileges:   []string{"ALL"},
		},
	}).Return(nil)

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
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}
	instances := []model.ValkeyInstance{
		{ID: "vk-1", Name: "cache", ShardID: &shardID, Port: 6379, Password: "vkpass", MaxMemoryMB: 128, Status: model.StatusActive},
	}
	users := []model.ValkeyUser{
		{ID: "vku-1", ValkeyInstanceID: "vk-1", Username: "app", Password: "apppass", Privileges: []string{"allcommands"}, KeyPattern: "*", Status: model.StatusActive},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("ListValkeyInstancesByShard", mock.Anything, shardID).Return(instances, nil)
	s.env.OnActivity("CreateValkeyInstanceOnNode", mock.Anything, activity.CreateValkeyInstanceOnNodeParams{
		NodeAddress: "node1:9090",
		Instance: activity.CreateValkeyInstanceParams{
			Name:        "cache",
			Port:        6379,
			Password:    "vkpass",
			MaxMemoryMB: 128,
		},
	}).Return(nil)
	s.env.OnActivity("ListValkeyUsersByInstanceID", mock.Anything, "vk-1").Return(users, nil)
	s.env.OnActivity("CreateValkeyUserOnNode", mock.Anything, activity.CreateValkeyUserOnNodeParams{
		NodeAddress: "node1:9090",
		User: activity.CreateValkeyUserParams{
			InstanceName: "cache",
			Port:         6379,
			Username:     "app",
			Password:     "apppass",
			Privileges:   []string{"allcommands"},
			KeyPattern:   "*",
		},
	}).Return(nil)

	s.env.ExecuteWorkflow(ConvergeShardWorkflow, ConvergeShardParams{ShardID: shardID})
	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
}

func (s *ConvergeShardWorkflowTestSuite) TestNoActiveNodes() {
	shardID := "shard-empty"
	shard := model.Shard{
		ID:   shardID,
		Role: model.ShardRoleWeb,
	}
	// Nodes with no gRPC address.
	nodes := []model.Node{
		{ID: "node-1", GRPCAddress: ""},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)

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
		{ID: "node-1", GRPCAddress: "node1:9090"},
	}
	// A provisioning tenant should be skipped.
	tenants := []model.Tenant{
		{ID: "tenant-active", Name: "active", UID: 1000, Status: model.StatusActive},
		{ID: "tenant-prov", Name: "provisioning", UID: 1001, Status: model.StatusProvisioning},
	}

	s.env.OnActivity("GetShardByID", mock.Anything, shardID).Return(&shard, nil)
	s.env.OnActivity("ListNodesByShard", mock.Anything, shardID).Return(nodes, nil)
	s.env.OnActivity("ListTenantsByShard", mock.Anything, shardID).Return(tenants, nil)

	// Only the active tenant gets CreateTenantOnNode.
	s.env.OnActivity("CreateTenantOnNode", mock.Anything, activity.CreateTenantOnNodeParams{
		NodeAddress: "node1:9090",
		Tenant:      activity.CreateTenantParams{ID: "tenant-active", Name: "active", UID: 1000},
	}).Return(nil)

	s.env.OnActivity("ListWebrootsByTenantID", mock.Anything, "tenant-active").Return([]model.Webroot{}, nil)
	s.env.OnActivity("ReloadNginxOnNode", mock.Anything, "node1:9090").Return(nil)

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

// ---------- Run ----------

func TestConvergeShardWorkflow(t *testing.T) {
	suite.Run(t, new(ConvergeShardWorkflowTestSuite))
}
