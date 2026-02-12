package activity

import (
	"context"
	"fmt"

	agentv1 "github.com/edvin/hosting/proto/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// NodeGRPC contains activities that wrap gRPC calls to the node agent.
type NodeGRPC struct {
	addr string
}

// NewNodeGRPC creates a new NodeGRPC activity struct.
func NewNodeGRPC(addr string) *NodeGRPC {
	return &NodeGRPC{addr: addr}
}

// CreateTenantParams holds parameters for the CreateTenant gRPC call.
type CreateTenantParams struct {
	ID          string
	Name        string
	UID         int
	SFTPEnabled bool
}

// CreateTenant calls the node agent to create a tenant.
func (a *NodeGRPC) CreateTenant(ctx context.Context, params CreateTenantParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateTenant(ctx, &agentv1.CreateTenantRequest{
		Tenant: &agentv1.TenantInfo{
			Id:          params.ID,
			Name:        params.Name,
			Uid:         int32(params.UID),
			SftpEnabled: params.SFTPEnabled,
		},
	})
	return err
}

// UpdateTenantParams holds parameters for the UpdateTenant gRPC call.
type UpdateTenantParams struct {
	ID          string
	Name        string
	UID         int
	SFTPEnabled bool
}

// UpdateTenant calls the node agent to update a tenant.
func (a *NodeGRPC) UpdateTenant(ctx context.Context, params UpdateTenantParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.UpdateTenant(ctx, &agentv1.UpdateTenantRequest{
		Tenant: &agentv1.TenantInfo{
			Id:          params.ID,
			Name:        params.Name,
			Uid:         int32(params.UID),
			SftpEnabled: params.SFTPEnabled,
		},
	})
	return err
}

// SuspendTenant calls the node agent to suspend a tenant.
func (a *NodeGRPC) SuspendTenant(ctx context.Context, name string) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.SuspendTenant(ctx, &agentv1.SuspendTenantRequest{
		TenantName: name,
	})
	return err
}

// UnsuspendTenant calls the node agent to unsuspend a tenant.
func (a *NodeGRPC) UnsuspendTenant(ctx context.Context, name string) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.UnsuspendTenant(ctx, &agentv1.UnsuspendTenantRequest{
		TenantName: name,
	})
	return err
}

// DeleteTenant calls the node agent to delete a tenant.
func (a *NodeGRPC) DeleteTenant(ctx context.Context, name string) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteTenant(ctx, &agentv1.DeleteTenantRequest{
		TenantName: name,
	})
	return err
}

// CreateWebrootParams holds parameters for the CreateWebroot gRPC call.
type CreateWebrootParams struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
	FQDNs          []FQDNParam
}

// FQDNParam represents an FQDN for gRPC webroot operations.
type FQDNParam struct {
	FQDN       string
	WebrootID  string
	SSLEnabled bool
}

// CreateWebroot calls the node agent to create a webroot.
func (a *NodeGRPC) CreateWebroot(ctx context.Context, params CreateWebrootParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	fqdns := make([]*agentv1.FQDNInfo, len(params.FQDNs))
	for i, f := range params.FQDNs {
		fqdns[i] = &agentv1.FQDNInfo{
			Fqdn:       f.FQDN,
			WebrootId:  f.WebrootID,
			SslEnabled: f.SSLEnabled,
		}
	}

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateWebroot(ctx, &agentv1.CreateWebrootRequest{
		Webroot: &agentv1.WebrootInfo{
			Id:             params.ID,
			TenantName:     params.TenantName,
			Name:           params.Name,
			Runtime:        params.Runtime,
			RuntimeVersion: params.RuntimeVersion,
			RuntimeConfig:  params.RuntimeConfig,
			PublicFolder:   params.PublicFolder,
		},
		Fqdns: fqdns,
	})
	return err
}

// UpdateWebrootParams holds parameters for the UpdateWebroot gRPC call.
type UpdateWebrootParams struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
	FQDNs          []FQDNParam
}

// UpdateWebroot calls the node agent to update a webroot.
func (a *NodeGRPC) UpdateWebroot(ctx context.Context, params UpdateWebrootParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	fqdns := make([]*agentv1.FQDNInfo, len(params.FQDNs))
	for i, f := range params.FQDNs {
		fqdns[i] = &agentv1.FQDNInfo{
			Fqdn:       f.FQDN,
			WebrootId:  f.WebrootID,
			SslEnabled: f.SSLEnabled,
		}
	}

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.UpdateWebroot(ctx, &agentv1.UpdateWebrootRequest{
		Webroot: &agentv1.WebrootInfo{
			Id:             params.ID,
			TenantName:     params.TenantName,
			Name:           params.Name,
			Runtime:        params.Runtime,
			RuntimeVersion: params.RuntimeVersion,
			RuntimeConfig:  params.RuntimeConfig,
			PublicFolder:   params.PublicFolder,
		},
		Fqdns: fqdns,
	})
	return err
}

// DeleteWebroot calls the node agent to delete a webroot.
func (a *NodeGRPC) DeleteWebroot(ctx context.Context, tenantName, webrootName string) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteWebroot(ctx, &agentv1.DeleteWebrootRequest{
		TenantName:  tenantName,
		WebrootName: webrootName,
	})
	return err
}

// ConfigureRuntimeParams holds parameters for the ConfigureRuntime gRPC call.
type ConfigureRuntimeParams struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
}

// ConfigureRuntime calls the node agent to configure a runtime for a webroot.
func (a *NodeGRPC) ConfigureRuntime(ctx context.Context, params ConfigureRuntimeParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.ConfigureRuntime(ctx, &agentv1.ConfigureRuntimeRequest{
		Webroot: &agentv1.WebrootInfo{
			Id:             params.ID,
			TenantName:     params.TenantName,
			Name:           params.Name,
			Runtime:        params.Runtime,
			RuntimeVersion: params.RuntimeVersion,
			RuntimeConfig:  params.RuntimeConfig,
			PublicFolder:   params.PublicFolder,
		},
	})
	return err
}

// ReloadNginx calls the node agent to reload the nginx configuration.
func (a *NodeGRPC) ReloadNginx(ctx context.Context) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.ReloadNginx(ctx, &agentv1.ReloadNginxRequest{})
	return err
}

// CreateDatabase calls the node agent to create a database.
func (a *NodeGRPC) CreateDatabase(ctx context.Context, name string) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateDatabase(ctx, &agentv1.CreateDatabaseRequest{
		Database: &agentv1.DatabaseInfo{
			Name: name,
		},
	})
	return err
}

// DeleteDatabase calls the node agent to delete a database.
func (a *NodeGRPC) DeleteDatabase(ctx context.Context, name string) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteDatabase(ctx, &agentv1.DeleteDatabaseRequest{
		DatabaseName: name,
	})
	return err
}

// CreateDatabaseUserParams holds parameters for the CreateDatabaseUser gRPC call.
type CreateDatabaseUserParams struct {
	DatabaseName string
	Username     string
	Password     string
	Privileges   []string
}

// CreateDatabaseUser calls the node agent to create a database user.
func (a *NodeGRPC) CreateDatabaseUser(ctx context.Context, params CreateDatabaseUserParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateDatabaseUser(ctx, &agentv1.CreateDatabaseUserRequest{
		User: &agentv1.DatabaseUserInfo{
			DatabaseName: params.DatabaseName,
			Username:     params.Username,
			Password:     params.Password,
			Privileges:   params.Privileges,
		},
	})
	return err
}

// UpdateDatabaseUserParams holds parameters for the UpdateDatabaseUser gRPC call.
type UpdateDatabaseUserParams struct {
	DatabaseName string
	Username     string
	Password     string
	Privileges   []string
}

// UpdateDatabaseUser calls the node agent to update a database user.
func (a *NodeGRPC) UpdateDatabaseUser(ctx context.Context, params UpdateDatabaseUserParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.UpdateDatabaseUser(ctx, &agentv1.UpdateDatabaseUserRequest{
		User: &agentv1.DatabaseUserInfo{
			DatabaseName: params.DatabaseName,
			Username:     params.Username,
			Password:     params.Password,
			Privileges:   params.Privileges,
		},
	})
	return err
}

// DeleteDatabaseUser calls the node agent to delete a database user.
func (a *NodeGRPC) DeleteDatabaseUser(ctx context.Context, dbName, username string) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteDatabaseUser(ctx, &agentv1.DeleteDatabaseUserRequest{
		DatabaseName: dbName,
		Username:     username,
	})
	return err
}

// CreateValkeyInstanceParams holds parameters for the CreateValkeyInstance gRPC call.
type CreateValkeyInstanceParams struct {
	Name        string
	Port        int
	Password    string
	MaxMemoryMB int
}

// CreateValkeyInstance calls the node agent to create a valkey instance.
func (a *NodeGRPC) CreateValkeyInstance(ctx context.Context, params CreateValkeyInstanceParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateValkeyInstance(ctx, &agentv1.CreateValkeyInstanceRequest{
		Instance: &agentv1.ValkeyInstanceInfo{
			Name:        params.Name,
			Port:        int32(params.Port),
			Password:    params.Password,
			MaxMemoryMb: int32(params.MaxMemoryMB),
		},
	})
	return err
}

// DeleteValkeyInstanceParams holds parameters for the DeleteValkeyInstance gRPC call.
type DeleteValkeyInstanceParams struct {
	Name string
	Port int
}

// DeleteValkeyInstance calls the node agent to delete a valkey instance.
func (a *NodeGRPC) DeleteValkeyInstance(ctx context.Context, params DeleteValkeyInstanceParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteValkeyInstance(ctx, &agentv1.DeleteValkeyInstanceRequest{
		InstanceName: params.Name,
		Port:         int32(params.Port),
	})
	return err
}

// CreateValkeyUserParams holds parameters for the CreateValkeyUser gRPC call.
type CreateValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
	Password     string
	Privileges   []string
	KeyPattern   string
}

// CreateValkeyUser calls the node agent to create a valkey user.
func (a *NodeGRPC) CreateValkeyUser(ctx context.Context, params CreateValkeyUserParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.CreateValkeyUser(ctx, &agentv1.CreateValkeyUserRequest{
		User: &agentv1.ValkeyUserInfo{
			InstanceName: params.InstanceName,
			Port:         int32(params.Port),
			Username:     params.Username,
			Password:     params.Password,
			Privileges:   params.Privileges,
			KeyPattern:   params.KeyPattern,
		},
	})
	return err
}

// UpdateValkeyUserParams holds parameters for the UpdateValkeyUser gRPC call.
type UpdateValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
	Password     string
	Privileges   []string
	KeyPattern   string
}

// UpdateValkeyUser calls the node agent to update a valkey user.
func (a *NodeGRPC) UpdateValkeyUser(ctx context.Context, params UpdateValkeyUserParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.UpdateValkeyUser(ctx, &agentv1.UpdateValkeyUserRequest{
		User: &agentv1.ValkeyUserInfo{
			InstanceName: params.InstanceName,
			Port:         int32(params.Port),
			Username:     params.Username,
			Password:     params.Password,
			Privileges:   params.Privileges,
			KeyPattern:   params.KeyPattern,
		},
	})
	return err
}

// DeleteValkeyUserParams holds parameters for the DeleteValkeyUser gRPC call.
type DeleteValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
}

// DeleteValkeyUser calls the node agent to delete a valkey user.
func (a *NodeGRPC) DeleteValkeyUser(ctx context.Context, params DeleteValkeyUserParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.DeleteValkeyUser(ctx, &agentv1.DeleteValkeyUserRequest{
		InstanceName: params.InstanceName,
		Port:         int32(params.Port),
		Username:     params.Username,
	})
	return err
}

// InstallCertificateParams holds parameters for the InstallCertificate gRPC call.
type InstallCertificateParams struct {
	FQDN     string
	CertPEM  string
	KeyPEM   string
	ChainPEM string
}

// InstallCertificate calls the node agent to install a TLS certificate.
func (a *NodeGRPC) InstallCertificate(ctx context.Context, params InstallCertificateParams) error {
	conn, err := grpc.NewClient(a.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial node agent: %w", err)
	}
	defer conn.Close()

	client := agentv1.NewNodeAgentClient(conn)
	_, err = client.InstallCertificate(ctx, &agentv1.InstallCertificateRequest{
		Certificate: &agentv1.CertificateInfo{
			Fqdn:     params.FQDN,
			CertPem:  params.CertPEM,
			KeyPem:   params.KeyPEM,
			ChainPem: params.ChainPEM,
		},
	})
	return err
}
