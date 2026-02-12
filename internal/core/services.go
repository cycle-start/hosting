package core

import (
	temporalclient "go.temporal.io/sdk/client"
)

type Services struct {
	PlatformConfig   *PlatformConfigService
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
	SFTPKey          *SFTPKeyService
	Backup           *BackupService
	APIKey           *APIKeyService
}

func NewServices(db DB, tc temporalclient.Client) *Services {
	return &Services{
		PlatformConfig:   NewPlatformConfigService(db),
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
		SFTPKey:          NewSFTPKeyService(db, tc),
		Backup:           NewBackupService(db, tc),
		APIKey:           NewAPIKeyService(db),
	}
}
