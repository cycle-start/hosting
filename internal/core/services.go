package core

import (
	temporalclient "go.temporal.io/sdk/client"
)

type Services struct {
	Dashboard        *DashboardService
	PlatformConfig   *PlatformConfigService
	Brand            *BrandService
	Region           *RegionService
	Cluster          *ClusterService
	ClusterLBAddress *ClusterLBAddressService
	Shard            *ShardService
	Node             *NodeService
	Tenant           *TenantService
	Webroot          *WebrootService
	FQDN             *FQDNService
	Certificate      *CertificateService
	Zone             *ZoneService
	ZoneRecord       *ZoneRecordService
	Database         *DatabaseService
	DatabaseUser     *DatabaseUserService
	EmailAccount     *EmailAccountService
	EmailAlias       *EmailAliasService
	EmailForward     *EmailForwardService
	EmailAutoReply   *EmailAutoReplyService
	ValkeyInstance   *ValkeyInstanceService
	ValkeyUser       *ValkeyUserService
	S3Bucket         *S3BucketService
	S3AccessKey      *S3AccessKeyService
	SSHKey           *SSHKeyService
	Backup           *BackupService
	APIKey           *APIKeyService
	OIDC             *OIDCService
	Search           *SearchService
}

func NewServices(db DB, tc temporalclient.Client, oidcIssuerURL string) *Services {
	return &Services{
		Dashboard:        NewDashboardService(db),
		PlatformConfig:   NewPlatformConfigService(db),
		Brand:            NewBrandService(db),
		Region:           NewRegionService(db),
		Cluster:          NewClusterService(db),
		ClusterLBAddress: NewClusterLBAddressService(db),
		Shard:            NewShardService(db, tc),
		Node:             NewNodeService(db),
		Tenant:           NewTenantService(db, tc),
		Webroot:          NewWebrootService(db, tc),
		FQDN:             NewFQDNService(db, tc),
		Certificate:      NewCertificateService(db, tc),
		Zone:             NewZoneService(db, tc),
		ZoneRecord:       NewZoneRecordService(db, tc),
		Database:         NewDatabaseService(db, tc),
		DatabaseUser:     NewDatabaseUserService(db, tc),
		EmailAccount:     NewEmailAccountService(db, tc),
		EmailAlias:       NewEmailAliasService(db, tc),
		EmailForward:     NewEmailForwardService(db, tc),
		EmailAutoReply:   NewEmailAutoReplyService(db, tc),
		ValkeyInstance:   NewValkeyInstanceService(db, tc),
		ValkeyUser:       NewValkeyUserService(db, tc),
		S3Bucket:         NewS3BucketService(db, tc),
		S3AccessKey:      NewS3AccessKeyService(db, tc),
		SSHKey:           NewSSHKeyService(db, tc),
		Backup:           NewBackupService(db, tc),
		APIKey:           NewAPIKeyService(db),
		OIDC:             NewOIDCService(db, oidcIssuerURL),
		Search:           NewSearchService(db),
	}
}
