package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/config"
	"github.com/edvin/hosting/internal/db"
	"github.com/edvin/hosting/internal/workflow"
)

const taskQueue = "hosting-tasks"

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	if err := cfg.Validate("worker"); err != nil {
		logger.Fatal().Err(err).Msg("invalid config")
	}

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		logger.Fatal().Str("level", cfg.LogLevel).Msg("invalid log level")
	}
	logger = logger.Level(level)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	corePool, err := db.NewCorePool(ctx, cfg.CoreDatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to core database")
	}
	defer corePool.Close()

	servicePool, err := db.NewServicePool(ctx, cfg.ServiceDatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to service database")
	}
	if servicePool != nil {
		defer servicePool.Close()
	}

	tlsConfig, err := cfg.TemporalTLS()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to configure temporal TLS")
	}
	dialOpts := temporalclient.Options{HostPort: cfg.TemporalAddress}
	if tlsConfig != nil {
		dialOpts.ConnectionOptions = temporalclient.ConnectionOptions{TLS: tlsConfig}
		logger.Info().Msg("temporal mTLS enabled")
	}
	tc, err := temporalclient.Dial(dialOpts)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to temporal")
	}
	defer tc.Close()

	w := worker.New(tc, taskQueue, worker.Options{})

	// Register activities
	coreDBActivities := activity.NewCoreDB(corePool)
	w.RegisterActivity(coreDBActivities)

	serviceDBActivities := activity.NewServiceDB(servicePool)
	w.RegisterActivity(serviceDBActivities)

	dnsActivities := activity.NewDNS(corePool, serviceDBActivities)
	w.RegisterActivity(dnsActivities)

	certActivities := activity.NewCertificateActivity(corePool)
	w.RegisterActivity(certActivities)

	acmeActivities := activity.NewACMEActivity(cfg.ACMEEmail, cfg.ACMEDirectoryURL)
	w.RegisterActivity(acmeActivities)

	lbActivities := activity.NewLB(corePool)
	w.RegisterActivity(lbActivities)

	migrateActivities := activity.NewMigrate(corePool)
	w.RegisterActivity(migrateActivities)

	stalwartActivities := activity.NewStalwart(corePool)
	w.RegisterActivity(stalwartActivities)

	// Register workflows
	w.RegisterWorkflow(workflow.CreateTenantWorkflow)
	w.RegisterWorkflow(workflow.UpdateTenantWorkflow)
	w.RegisterWorkflow(workflow.SuspendTenantWorkflow)
	w.RegisterWorkflow(workflow.UnsuspendTenantWorkflow)
	w.RegisterWorkflow(workflow.DeleteTenantWorkflow)
	w.RegisterWorkflow(workflow.CreateWebrootWorkflow)
	w.RegisterWorkflow(workflow.UpdateWebrootWorkflow)
	w.RegisterWorkflow(workflow.DeleteWebrootWorkflow)
	w.RegisterWorkflow(workflow.BindFQDNWorkflow)
	w.RegisterWorkflow(workflow.UnbindFQDNWorkflow)
	w.RegisterWorkflow(workflow.ProvisionLECertWorkflow)
	w.RegisterWorkflow(workflow.UploadCustomCertWorkflow)
	w.RegisterWorkflow(workflow.RenewLECertWorkflow)
	w.RegisterWorkflow(workflow.CleanupExpiredCertsWorkflow)
	w.RegisterWorkflow(workflow.CreateZoneWorkflow)
	w.RegisterWorkflow(workflow.DeleteZoneWorkflow)
	w.RegisterWorkflow(workflow.CreateZoneRecordWorkflow)
	w.RegisterWorkflow(workflow.UpdateZoneRecordWorkflow)
	w.RegisterWorkflow(workflow.DeleteZoneRecordWorkflow)
	w.RegisterWorkflow(workflow.CreateDatabaseWorkflow)
	w.RegisterWorkflow(workflow.DeleteDatabaseWorkflow)
	w.RegisterWorkflow(workflow.CreateDatabaseUserWorkflow)
	w.RegisterWorkflow(workflow.UpdateDatabaseUserWorkflow)
	w.RegisterWorkflow(workflow.DeleteDatabaseUserWorkflow)
	w.RegisterWorkflow(workflow.UpdateServiceHostnamesWorkflow)
	w.RegisterWorkflow(workflow.MigrateTenantWorkflow)
	w.RegisterWorkflow(workflow.MigrateDatabaseWorkflow)
	w.RegisterWorkflow(workflow.MigrateValkeyInstanceWorkflow)
	w.RegisterWorkflow(workflow.CreateEmailAccountWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailAccountWorkflow)
	w.RegisterWorkflow(workflow.CreateEmailAliasWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailAliasWorkflow)
	w.RegisterWorkflow(workflow.CreateEmailForwardWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailForwardWorkflow)
	w.RegisterWorkflow(workflow.UpdateEmailAutoReplyWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailAutoReplyWorkflow)
	w.RegisterWorkflow(workflow.CreateValkeyInstanceWorkflow)
	w.RegisterWorkflow(workflow.DeleteValkeyInstanceWorkflow)
	w.RegisterWorkflow(workflow.CreateValkeyUserWorkflow)
	w.RegisterWorkflow(workflow.UpdateValkeyUserWorkflow)
	w.RegisterWorkflow(workflow.DeleteValkeyUserWorkflow)
	w.RegisterWorkflow(workflow.AddSFTPKeyWorkflow)
	w.RegisterWorkflow(workflow.RemoveSFTPKeyWorkflow)
	w.RegisterWorkflow(workflow.ConvergeShardWorkflow)
	w.RegisterWorkflow(workflow.CreateBackupWorkflow)
	w.RegisterWorkflow(workflow.RestoreBackupWorkflow)
	w.RegisterWorkflow(workflow.DeleteBackupWorkflow)

	go func() {
		logger.Info().Str("taskQueue", taskQueue).Msg("starting temporal worker")
		if err := w.Run(worker.InterruptCh()); err != nil {
			logger.Fatal().Err(err).Msg("worker failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down worker")
	cancel()
}
