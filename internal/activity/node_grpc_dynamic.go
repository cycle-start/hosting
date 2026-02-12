package activity

import (
	"context"
	"fmt"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NodeGRPCDynamic contains activities that route gRPC calls to specific node addresses.
// This replaces the hardcoded single-address NodeGRPC for shard-aware routing.
type NodeGRPCDynamic struct {
	db DB // for looking up node addresses by shard
}

// NewNodeGRPCDynamic creates a new NodeGRPCDynamic activity struct.
func NewNodeGRPCDynamic(db DB) *NodeGRPCDynamic {
	return &NodeGRPCDynamic{db: db}
}

func dialNode(addr string) (*grpc.ClientConn, error) {
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// CreateTenantOnNodeParams holds parameters for creating a tenant on a specific node.
type CreateTenantOnNodeParams struct {
	NodeAddress string            `json:"node_address"`
	Tenant      CreateTenantParams `json:"tenant"`
}

// CreateTenantOnNode calls a specific node agent to create a tenant.
func (a *NodeGRPCDynamic) CreateTenantOnNode(ctx context.Context, params CreateTenantOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateTenant(ctx, &agentv1.CreateTenantRequest{
		Tenant: &agentv1.TenantInfo{
			Id:          params.Tenant.ID,
			Name:        params.Tenant.Name,
			Uid:         int32(params.Tenant.UID),
			SftpEnabled: params.Tenant.SFTPEnabled,
		},
	})
	return err
}

// DeleteTenantOnNodeParams holds parameters for deleting a tenant on a specific node.
type DeleteTenantOnNodeParams struct {
	NodeAddress string `json:"node_address"`
	TenantName  string `json:"tenant_name"`
}

// DeleteTenantOnNode calls a specific node agent to delete a tenant.
func (a *NodeGRPCDynamic) DeleteTenantOnNode(ctx context.Context, params DeleteTenantOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteTenant(ctx, &agentv1.DeleteTenantRequest{
		TenantName: params.TenantName,
	})
	return err
}

// CreateWebrootOnNodeParams holds parameters for creating a webroot on a specific node.
type CreateWebrootOnNodeParams struct {
	NodeAddress string             `json:"node_address"`
	Webroot     CreateWebrootParams `json:"webroot"`
}

// CreateWebrootOnNode calls a specific node agent to create a webroot.
func (a *NodeGRPCDynamic) CreateWebrootOnNode(ctx context.Context, params CreateWebrootOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	fqdns := make([]*agentv1.FQDNInfo, len(params.Webroot.FQDNs))
	for i, f := range params.Webroot.FQDNs {
		fqdns[i] = &agentv1.FQDNInfo{
			Fqdn:       f.FQDN,
			WebrootId:  f.WebrootID,
			SslEnabled: f.SSLEnabled,
		}
	}

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateWebroot(ctx, &agentv1.CreateWebrootRequest{
		Webroot: &agentv1.WebrootInfo{
			Id:             params.Webroot.ID,
			TenantName:     params.Webroot.TenantName,
			Name:           params.Webroot.Name,
			Runtime:        params.Webroot.Runtime,
			RuntimeVersion: params.Webroot.RuntimeVersion,
			RuntimeConfig:  params.Webroot.RuntimeConfig,
			PublicFolder:   params.Webroot.PublicFolder,
		},
		Fqdns: fqdns,
	})
	return err
}

// ReloadNginxOnNode calls a specific node agent to reload nginx.
func (a *NodeGRPCDynamic) ReloadNginxOnNode(ctx context.Context, nodeAddress string) error {
	conn, err := dialNode(nodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", nodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.ReloadNginx(ctx, &agentv1.ReloadNginxRequest{})
	return err
}

// --------------------------------------------------------------------------
// Database on-node activities
// --------------------------------------------------------------------------

// CreateDatabaseOnNodeParams holds parameters for creating a database on a specific node.
type CreateDatabaseOnNodeParams struct {
	NodeAddress string `json:"node_address"`
	Name        string `json:"name"`
}

// CreateDatabaseOnNode calls a specific node agent to create a database.
func (a *NodeGRPCDynamic) CreateDatabaseOnNode(ctx context.Context, params CreateDatabaseOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateDatabase(ctx, &agentv1.CreateDatabaseRequest{
		Database: &agentv1.DatabaseInfo{Name: params.Name},
	})
	return err
}

// CreateDatabaseUserOnNodeParams holds parameters for creating a database user on a specific node.
type CreateDatabaseUserOnNodeParams struct {
	NodeAddress string               `json:"node_address"`
	User        CreateDatabaseUserParams `json:"user"`
}

// CreateDatabaseUserOnNode calls a specific node agent to create a database user.
func (a *NodeGRPCDynamic) CreateDatabaseUserOnNode(ctx context.Context, params CreateDatabaseUserOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateDatabaseUser(ctx, &agentv1.CreateDatabaseUserRequest{
		User: &agentv1.DatabaseUserInfo{
			DatabaseName: params.User.DatabaseName,
			Username:     params.User.Username,
			Password:     params.User.Password,
			Privileges:   params.User.Privileges,
		},
	})
	return err
}

// --------------------------------------------------------------------------
// Valkey on-node activities
// --------------------------------------------------------------------------

// CreateValkeyInstanceOnNodeParams holds parameters for creating a valkey instance on a specific node.
type CreateValkeyInstanceOnNodeParams struct {
	NodeAddress string                   `json:"node_address"`
	Instance    CreateValkeyInstanceParams `json:"instance"`
}

// CreateValkeyInstanceOnNode calls a specific node agent to create a valkey instance.
func (a *NodeGRPCDynamic) CreateValkeyInstanceOnNode(ctx context.Context, params CreateValkeyInstanceOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateValkeyInstance(ctx, &agentv1.CreateValkeyInstanceRequest{
		Instance: &agentv1.ValkeyInstanceInfo{
			Name:        params.Instance.Name,
			Port:        int32(params.Instance.Port),
			Password:    params.Instance.Password,
			MaxMemoryMb: int32(params.Instance.MaxMemoryMB),
		},
	})
	return err
}

// CreateValkeyUserOnNodeParams holds parameters for creating a valkey user on a specific node.
type CreateValkeyUserOnNodeParams struct {
	NodeAddress string                `json:"node_address"`
	User        CreateValkeyUserParams `json:"user"`
}

// CreateValkeyUserOnNode calls a specific node agent to create a valkey user.
func (a *NodeGRPCDynamic) CreateValkeyUserOnNode(ctx context.Context, params CreateValkeyUserOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateValkeyUser(ctx, &agentv1.CreateValkeyUserRequest{
		User: &agentv1.ValkeyUserInfo{
			InstanceName: params.User.InstanceName,
			Port:         int32(params.User.Port),
			Username:     params.User.Username,
			Password:     params.User.Password,
			Privileges:   params.User.Privileges,
			KeyPattern:   params.User.KeyPattern,
		},
	})
	return err
}

// DeleteWebrootOnNodeParams holds parameters for deleting a webroot on a specific node.
type DeleteWebrootOnNodeParams struct {
	NodeAddress string `json:"node_address"`
	TenantName  string `json:"tenant_name"`
	WebrootName string `json:"webroot_name"`
}

// DeleteWebrootOnNode calls a specific node agent to delete a webroot.
func (a *NodeGRPCDynamic) DeleteWebrootOnNode(ctx context.Context, params DeleteWebrootOnNodeParams) error {
	conn, err := dialNode(params.NodeAddress)
	if err != nil {
		return fmt.Errorf("dial node %s: %w", params.NodeAddress, err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteWebroot(ctx, &agentv1.DeleteWebrootRequest{
		TenantName:  params.TenantName,
		WebrootName: params.WebrootName,
	})
	return err
}
