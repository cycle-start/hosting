package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/rs/zerolog"
	temporalclient "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/config"
	"github.com/edvin/hosting/internal/db"
	"github.com/edvin/hosting/internal/logging"
	"github.com/edvin/hosting/internal/metrics"
	"github.com/edvin/hosting/internal/workflow"
)

const taskQueue = "hosting-tasks"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate("worker"); err != nil {
		fmt.Fprintf(os.Stderr, "invalid config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.NewLogger(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	corePool, err := db.NewCorePool(ctx, cfg.CoreDatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to core database")
	}
	defer corePool.Close()

	powerdnsPool, err := db.NewPowerDNSPool(ctx, cfg.PowerDNSDatabaseURL)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to connect to powerdns database")
	}
	if powerdnsPool != nil {
		defer powerdnsPool.Close()
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

	powerdnsDBActivities := activity.NewPowerDNSDB(powerdnsPool)
	w.RegisterActivity(powerdnsDBActivities)

	dnsActivities := activity.NewDNS(corePool, powerdnsDBActivities)
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
	w.RegisterWorkflow(workflow.TenantProvisionWorkflow)
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
	w.RegisterWorkflow(workflow.CreateS3BucketWorkflow)
	w.RegisterWorkflow(workflow.UpdateS3BucketWorkflow)
	w.RegisterWorkflow(workflow.DeleteS3BucketWorkflow)
	w.RegisterWorkflow(workflow.CreateS3AccessKeyWorkflow)
	w.RegisterWorkflow(workflow.DeleteS3AccessKeyWorkflow)
	w.RegisterWorkflow(workflow.ConvergeShardWorkflow)
	w.RegisterWorkflow(workflow.CreateBackupWorkflow)
	w.RegisterWorkflow(workflow.RestoreBackupWorkflow)
	w.RegisterWorkflow(workflow.DeleteBackupWorkflow)
	w.RegisterWorkflow(workflow.CleanupAuditLogsWorkflow)
	w.RegisterWorkflow(workflow.CleanupOldBackupsWorkflow)

	if cfg.MetricsAddr != "" {
		metricsSrv := metrics.NewServer(cfg.MetricsAddr)
		go func() {
			logger.Info().Str("addr", cfg.MetricsAddr).Msg("starting metrics server")
			if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error().Err(err).Msg("metrics server failed")
			}
		}()
	}

	go func() {
		logger.Info().Str("taskQueue", taskQueue).Msg("starting temporal worker")
		if err := w.Run(worker.InterruptCh()); err != nil {
			logger.Fatal().Err(err).Msg("worker failed")
		}
	}()

	// Register cron schedules. Errors for already-existing schedules are
	// ignored so that re-deploys do not fail.
	registerCronSchedules(ctx, tc, taskQueue, cfg, logger)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("shutting down worker")
	cancel()
}

type cronSchedule struct {
	id       string
	cron     string
	workflow interface{}
	args     []interface{}
}

func registerCronSchedules(ctx context.Context, tc temporalclient.Client, taskQueue string, cfg *config.Config, logger zerolog.Logger) {
	schedules := []cronSchedule{
		{
			id:       "cert-renewal-cron",
			cron:     "0 2 * * *",
			workflow: workflow.RenewLECertWorkflow,
		},
		{
			id:       "cert-cleanup-cron",
			cron:     "0 3 * * *",
			workflow: workflow.CleanupExpiredCertsWorkflow,
		},
		{
			id:       "audit-log-retention-cron",
			cron:     "0 4 * * *",
			workflow: workflow.CleanupAuditLogsWorkflow,
			args:     []interface{}{cfg.AuditLogRetentionDays},
		},
		{
			id:       "backup-retention-cron",
			cron:     "0 5 * * *",
			workflow: workflow.CleanupOldBackupsWorkflow,
			args:     []interface{}{cfg.BackupRetentionDays},
		},
	}

	scheduleClient := tc.ScheduleClient()

	for _, s := range schedules {
		_, err := scheduleClient.Create(ctx, temporalclient.ScheduleOptions{
			ID: s.id,
			Spec: temporalclient.ScheduleSpec{
				CronExpressions: []string{s.cron},
			},
			Action: &temporalclient.ScheduleWorkflowAction{
				ID:        s.id,
				Workflow:  s.workflow,
				Args:      s.args,
				TaskQueue: taskQueue,
			},
		})
		if err != nil {
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "AlreadyExists") || strings.Contains(err.Error(), "already registered") {
				logger.Info().Str("id", s.id).Msg("cron schedule already exists, skipping")
			} else {
				logger.Fatal().Err(err).Str("id", s.id).Msg("failed to create cron schedule")
			}
		} else {
			logger.Info().Str("id", s.id).Str("cron", s.cron).Msg("created cron schedule")
		}
	}
}
