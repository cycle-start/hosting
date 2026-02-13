package agent

// TenantInfo holds the information needed to manage a tenant on a node.
type TenantInfo struct {
	ID          string
	Name        string
	UID         int32
	SFTPEnabled bool
	SSHEnabled  bool
}

// FQDNInfo holds the FQDN configuration for nginx.
type FQDNInfo struct {
	FQDN       string
	WebrootID  string
	SSLEnabled bool
}

// CertificateInfo holds SSL certificate data for installation.
type CertificateInfo struct {
	FQDN     string
	CertPEM  string
	KeyPEM   string
	ChainPEM string
}
