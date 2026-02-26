package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	temporalclient "go.temporal.io/sdk/client"

	"github.com/edvin/hosting/internal/model"
)

type Services struct {
	pool               *pgxpool.Pool
	tc                 temporalclient.Client
	oidcIssuerURL      string
	secretEncryptionKey string

	Dashboard          *DashboardService
	PlatformConfig     *PlatformConfigService
	Brand              *BrandService
	Region             *RegionService
	Cluster            *ClusterService
	ClusterLBAddress   *ClusterLBAddressService
	Shard              *ShardService
	Node               *NodeService
	Tenant             *TenantService
	Subscription       *SubscriptionService
	Webroot            *WebrootService
	WebrootEnvVar      *WebrootEnvVarService
	FQDN               *FQDNService
	Certificate        *CertificateService
	Zone               *ZoneService
	ZoneRecord         *ZoneRecordService
	Database           *DatabaseService
	DatabaseUser       *DatabaseUserService
	EmailAccount       *EmailAccountService
	EmailAlias         *EmailAliasService
	EmailForward       *EmailForwardService
	EmailAutoReply     *EmailAutoReplyService
	ValkeyInstance     *ValkeyInstanceService
	ValkeyUser         *ValkeyUserService
	S3Bucket           *S3BucketService
	S3AccessKey        *S3AccessKeyService
	SSHKey             *SSHKeyService
	TenantEgressRule   *TenantEgressRuleService
	Backup             *BackupService
	CronJob            *CronJobService
	Daemon             *DaemonService
	APIKey             *APIKeyService
	OIDC               *OIDCService
	Search             *SearchService
	DesiredState       *DesiredStateService
	NodeHealth         *NodeHealthService
	Incident           *IncidentService
	CapabilityGap      *CapabilityGapService
	WireGuardPeer      *WireGuardPeerService
}

func NewServices(db *pgxpool.Pool, tc temporalclient.Client, oidcIssuerURL string, secretEncryptionKey string) *Services {
	svc := newServicesFromDB(db, tc, oidcIssuerURL, secretEncryptionKey)
	svc.pool = db
	return svc
}

func newServicesFromDB(db DB, tc temporalclient.Client, oidcIssuerURL string, secretEncryptionKey string) *Services {
	return &Services{
		tc:                  tc,
		oidcIssuerURL:       oidcIssuerURL,
		secretEncryptionKey: secretEncryptionKey,

		Dashboard:          NewDashboardService(db),
		PlatformConfig:     NewPlatformConfigService(db),
		Brand:              NewBrandService(db),
		Region:             NewRegionService(db),
		Cluster:            NewClusterService(db),
		ClusterLBAddress:   NewClusterLBAddressService(db),
		Shard:              NewShardService(db, tc),
		Node:               NewNodeService(db),
		Tenant:             NewTenantService(db, tc),
		Subscription:       NewSubscriptionService(db, tc),
		Webroot:            NewWebrootService(db, tc),
		WebrootEnvVar:      NewWebrootEnvVarService(db, tc, secretEncryptionKey),
		FQDN:               NewFQDNService(db, tc),
		Certificate:        NewCertificateService(db, tc),
		Zone:               NewZoneService(db, tc),
		ZoneRecord:         NewZoneRecordService(db, tc),
		Database:           NewDatabaseService(db, tc),
		DatabaseUser:       NewDatabaseUserService(db, tc),
		EmailAccount:       NewEmailAccountService(db, tc),
		EmailAlias:         NewEmailAliasService(db, tc),
		EmailForward:       NewEmailForwardService(db, tc),
		EmailAutoReply:     NewEmailAutoReplyService(db, tc),
		ValkeyInstance:     NewValkeyInstanceService(db, tc),
		ValkeyUser:         NewValkeyUserService(db, tc),
		S3Bucket:           NewS3BucketService(db, tc),
		S3AccessKey:        NewS3AccessKeyService(db, tc),
		SSHKey:             NewSSHKeyService(db, tc),
		TenantEgressRule:   NewTenantEgressRuleService(db, tc),
		Backup:             NewBackupService(db, tc),
		CronJob:            NewCronJobService(db, tc),
		Daemon:             NewDaemonService(db, tc),
		APIKey:             NewAPIKeyService(db),
		OIDC:               NewOIDCService(db, oidcIssuerURL),
		Search:             NewSearchService(db),
		DesiredState:       NewDesiredStateService(db, secretEncryptionKey),
		NodeHealth:         NewNodeHealthService(db),
		Incident:           NewIncidentService(db),
		CapabilityGap:      NewCapabilityGapService(db),
		WireGuardPeer:      NewWireGuardPeerService(db, tc, ""),
	}
}

// WithTx executes fn inside a database transaction. All service operations
// within fn use the transaction. The transaction is committed if fn returns nil,
// rolled back otherwise.
func (s *Services) WithTx(ctx context.Context, fn func(tx *Services) error) error {
	if s.pool == nil {
		return fmt.Errorf("WithTx: no pool available (already in a transaction?)")
	}
	pgxTx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer pgxTx.Rollback(ctx)

	txSvc := newServicesFromDB(pgxTx, s.tc, s.oidcIssuerURL, s.secretEncryptionKey)
	if err := fn(txSvc); err != nil {
		return err
	}
	return pgxTx.Commit(ctx)
}

// SignalProvision routes a workflow task through the per-tenant entity workflow.
// Used by handlers after committing a transaction to kick off provisioning.
func (s *Services) SignalProvision(ctx context.Context, tenantID string, task model.ProvisionTask) error {
	return signalProvision(ctx, s.tc, s.pool, tenantID, task)
}
