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
	"github.com/edvin/hosting/internal/deployer"
	"github.com/edvin/hosting/internal/workflow"
)

const taskQueue = "hosting-tasks"

func main() {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

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

	tc, err := temporalclient.Dial(temporalclient.Options{
		HostPort: cfg.TemporalAddress,
	})
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

	nodeGRPCActivities := activity.NewNodeGRPC(cfg.NodeAgentAddr)
	w.RegisterActivity(nodeGRPCActivities)

	nodeGRPCDynamicActivities := activity.NewNodeGRPCDynamic(corePool)
	w.RegisterActivity(nodeGRPCDynamicActivities)

	dnsActivities := activity.NewDNS(corePool, serviceDBActivities)
	w.RegisterActivity(dnsActivities)

	certActivities := activity.NewCertificateActivity(corePool)
	w.RegisterActivity(certActivities)

	var dep deployer.Deployer
	switch cfg.Deployer {
	case "docker":
		dep = deployer.NewDockerDeployer()
	default:
		logger.Fatal().Str("deployer", cfg.Deployer).Msg("unknown deployer")
	}
	deployActivities := activity.NewDeploy(dep)
	w.RegisterActivity(deployActivities)

	lbActivities := activity.NewLB(dep, corePool)
	w.RegisterActivity(lbActivities)

	migrateActivities := activity.NewMigrate(corePool)
	w.RegisterActivity(migrateActivities)

	stalwartActivities := activity.NewStalwart(corePool)
	w.RegisterActivity(stalwartActivities)

	clusterActivities := activity.NewCluster(dep, corePool)
	w.RegisterActivity(clusterActivities)

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
	w.RegisterWorkflow(workflow.ProvisionNodeWorkflow)
	w.RegisterWorkflow(workflow.DecommissionNodeWorkflow)
	w.RegisterWorkflow(workflow.RollingUpdateWorkflow)
	w.RegisterWorkflow(workflow.MigrateTenantWorkflow)
	w.RegisterWorkflow(workflow.CreateEmailAccountWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailAccountWorkflow)
	w.RegisterWorkflow(workflow.CreateEmailAliasWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailAliasWorkflow)
	w.RegisterWorkflow(workflow.CreateEmailForwardWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailForwardWorkflow)
	w.RegisterWorkflow(workflow.UpdateEmailAutoReplyWorkflow)
	w.RegisterWorkflow(workflow.DeleteEmailAutoReplyWorkflow)
	w.RegisterWorkflow(workflow.ProvisionClusterWorkflow)
	w.RegisterWorkflow(workflow.DecommissionClusterWorkflow)
	w.RegisterWorkflow(workflow.CreateValkeyInstanceWorkflow)
	w.RegisterWorkflow(workflow.DeleteValkeyInstanceWorkflow)
	w.RegisterWorkflow(workflow.CreateValkeyUserWorkflow)
	w.RegisterWorkflow(workflow.UpdateValkeyUserWorkflow)
	w.RegisterWorkflow(workflow.DeleteValkeyUserWorkflow)
	w.RegisterWorkflow(workflow.ConvergeShardWorkflow)

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
