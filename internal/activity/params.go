package activity

// CreateTenantParams holds parameters for creating a tenant on a node.
type CreateTenantParams struct {
	ID          string
	Name        string
	UID         int
	SFTPEnabled bool
}

// UpdateTenantParams holds parameters for updating a tenant on a node.
type UpdateTenantParams struct {
	ID          string
	Name        string
	UID         int
	SFTPEnabled bool
}

// FQDNParam represents an FQDN for webroot operations.
type FQDNParam struct {
	FQDN       string
	WebrootID  string
	SSLEnabled bool
}

// CreateWebrootParams holds parameters for creating a webroot on a node.
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

// UpdateWebrootParams holds parameters for updating a webroot on a node.
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

// ConfigureRuntimeParams holds parameters for configuring a runtime on a node.
type ConfigureRuntimeParams struct {
	ID             string
	TenantName     string
	Name           string
	Runtime        string
	RuntimeVersion string
	RuntimeConfig  string
	PublicFolder   string
}

// CreateDatabaseUserParams holds parameters for creating a database user on a node.
type CreateDatabaseUserParams struct {
	DatabaseName string
	Username     string
	Password     string
	Privileges   []string
}

// UpdateDatabaseUserParams holds parameters for updating a database user on a node.
type UpdateDatabaseUserParams struct {
	DatabaseName string
	Username     string
	Password     string
	Privileges   []string
}

// CreateValkeyInstanceParams holds parameters for creating a Valkey instance on a node.
type CreateValkeyInstanceParams struct {
	Name        string
	Port        int
	Password    string
	MaxMemoryMB int
}

// DeleteValkeyInstanceParams holds parameters for deleting a Valkey instance on a node.
type DeleteValkeyInstanceParams struct {
	Name string
	Port int
}

// CreateValkeyUserParams holds parameters for creating a Valkey user on a node.
type CreateValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
	Password     string
	Privileges   []string
	KeyPattern   string
}

// UpdateValkeyUserParams holds parameters for updating a Valkey user on a node.
type UpdateValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
	Password     string
	Privileges   []string
	KeyPattern   string
}

// DeleteValkeyUserParams holds parameters for deleting a Valkey user on a node.
type DeleteValkeyUserParams struct {
	InstanceName string
	Port         int
	Username     string
}

// InstallCertificateParams holds parameters for installing a certificate on a node.
type InstallCertificateParams struct {
	FQDN     string
	CertPEM  string
	KeyPEM   string
	ChainPEM string
}
