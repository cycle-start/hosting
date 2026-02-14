package activity

import "github.com/edvin/hosting/internal/model"

// WebrootContext bundles all data needed by webroot workflows.
type WebrootContext struct {
	Webroot model.Webroot `json:"webroot"`
	Tenant  model.Tenant  `json:"tenant"`
	Nodes   []model.Node  `json:"nodes"`
	FQDNs   []model.FQDN  `json:"fqdns"`
}

// FQDNContext bundles all data needed by FQDN workflows.
type FQDNContext struct {
	FQDN        model.FQDN              `json:"fqdn"`
	Webroot     model.Webroot            `json:"webroot"`
	Tenant      model.Tenant             `json:"tenant"`
	Shard       model.Shard              `json:"shard"`
	Nodes       []model.Node             `json:"nodes"`
	LBAddresses []model.ClusterLBAddress `json:"lb_addresses"`
	LBNodes     []model.Node             `json:"lb_nodes"`
}

// DatabaseUserContext bundles all data needed by database user workflows.
type DatabaseUserContext struct {
	User     model.DatabaseUser `json:"user"`
	Database model.Database     `json:"database"`
	Nodes    []model.Node       `json:"nodes"`
}

// ValkeyUserContext bundles all data needed by Valkey user workflows.
type ValkeyUserContext struct {
	User     model.ValkeyUser     `json:"user"`
	Instance model.ValkeyInstance `json:"instance"`
	Nodes    []model.Node         `json:"nodes"`
}

// ZoneRecordContext bundles all data needed by zone record workflows.
type ZoneRecordContext struct {
	Record   model.ZoneRecord `json:"record"`
	ZoneName string           `json:"zone_name"`
}

// BackupContext bundles all data needed by backup workflows.
type BackupContext struct {
	Backup model.Backup `json:"backup"`
	Tenant model.Tenant `json:"tenant"`
	Nodes  []model.Node `json:"nodes"`
}

// S3AccessKeyContext bundles all data needed by S3 access key workflows.
type S3AccessKeyContext struct {
	Key    model.S3AccessKey `json:"key"`
	Bucket model.S3Bucket    `json:"bucket"`
	Nodes  []model.Node      `json:"nodes"`
}

// StalwartContext bundles Stalwart connection info resolved from the cluster config,
// plus the FQDN fields needed by email account workflows.
type StalwartContext struct {
	StalwartURL   string `json:"stalwart_url"`
	StalwartToken string `json:"stalwart_token"`
	MailHostname  string `json:"mail_hostname"`
	FQDNID        string `json:"fqdn_id"`
	FQDN          string `json:"fqdn"`
}
