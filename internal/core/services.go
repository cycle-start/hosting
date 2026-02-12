package core

import (
	temporalclient "go.temporal.io/sdk/client"
)

type Services struct {
	PlatformConfig        *PlatformConfigService
	Region                *RegionService
	Cluster               *ClusterService
	ClusterLBAddress      *ClusterLBAddressService
	Shard                 *ShardService
	Node                  *NodeService
	Tenant                *TenantService
	Webroot               *WebrootService
	FQDN                  *FQDNService
	Certificate           *CertificateService
	Zone                  *ZoneService
	ZoneRecord            *ZoneRecordService
	Database              *DatabaseService
	DatabaseUser          *DatabaseUserService
	HostMachine           *HostMachineService
	NodeProfile           *NodeProfileService
	NodeDeployment        *NodeDeploymentService
	InfrastructureService *InfrastructureServiceService
	EmailAccount          *EmailAccountService
	EmailAlias            *EmailAliasService
	EmailForward          *EmailForwardService
	EmailAutoReply        *EmailAutoReplyService
	ValkeyInstance        *ValkeyInstanceService
	ValkeyUser            *ValkeyUserService
}

func NewServices(db DB, tc temporalclient.Client) *Services {
	return &Services{
		PlatformConfig:        NewPlatformConfigService(db),
		Region:                NewRegionService(db),
		Cluster:               NewClusterService(db, tc),
		ClusterLBAddress:      NewClusterLBAddressService(db),
		Shard:                 NewShardService(db, tc),
		Node:                  NewNodeService(db, tc),
		Tenant:                NewTenantService(db, tc),
		Webroot:               NewWebrootService(db, tc),
		FQDN:                  NewFQDNService(db, tc),
		Certificate:           NewCertificateService(db, tc),
		Zone:                  NewZoneService(db, tc),
		ZoneRecord:            NewZoneRecordService(db, tc),
		Database:              NewDatabaseService(db, tc),
		DatabaseUser:          NewDatabaseUserService(db, tc),
		HostMachine:           NewHostMachineService(db),
		NodeProfile:           NewNodeProfileService(db),
		NodeDeployment:        NewNodeDeploymentService(db),
		InfrastructureService: NewInfrastructureServiceService(db),
		EmailAccount:          NewEmailAccountService(db, tc),
		EmailAlias:            NewEmailAliasService(db, tc),
		EmailForward:          NewEmailForwardService(db, tc),
		EmailAutoReply:        NewEmailAutoReplyService(db, tc),
		ValkeyInstance:        NewValkeyInstanceService(db, tc),
		ValkeyUser:            NewValkeyUserService(db, tc),
	}
}
