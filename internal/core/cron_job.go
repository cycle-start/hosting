package core

import (
	"context"
	"fmt"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type CronJobService struct {
	db DB
	tc temporalclient.Client
}

func NewCronJobService(db DB, tc temporalclient.Client) *CronJobService {
	return &CronJobService{db: db, tc: tc}
}

func (s *CronJobService) Create(ctx context.Context, cronJob *model.CronJob) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO cron_jobs (id, tenant_id, webroot_id, name, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		cronJob.ID, cronJob.TenantID, cronJob.WebrootID, cronJob.Name, cronJob.Schedule,
		cronJob.Command, cronJob.WorkingDirectory, cronJob.Enabled, cronJob.TimeoutSeconds,
		cronJob.MaxMemoryMB, cronJob.Status, cronJob.CreatedAt, cronJob.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert cron job: %w", err)
	}

	if err := signalProvision(ctx, s.tc, cronJob.TenantID, model.ProvisionTask{
		WorkflowName: "CreateCronJobWorkflow",
		WorkflowID:   workflowID("cron-job", cronJob.Name, cronJob.ID),
		Arg:          cronJob.ID,
		ResourceType: "cron_job",
		ResourceID:   cronJob.ID,
	}); err != nil {
		return fmt.Errorf("start CreateCronJobWorkflow: %w", err)
	}

	return nil
}

func (s *CronJobService) GetByID(ctx context.Context, id string) (*model.CronJob, error) {
	var c model.CronJob
	err := s.db.QueryRow(ctx,
		`SELECT id, tenant_id, webroot_id, name, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, status, status_message, created_at, updated_at
		 FROM cron_jobs WHERE id = $1`, id,
	).Scan(&c.ID, &c.TenantID, &c.WebrootID, &c.Name, &c.Schedule, &c.Command,
		&c.WorkingDirectory, &c.Enabled, &c.TimeoutSeconds, &c.MaxMemoryMB,
		&c.Status, &c.StatusMessage, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get cron job %s: %w", id, err)
	}
	return &c, nil
}

func (s *CronJobService) ListByWebroot(ctx context.Context, webrootID string, limit int, cursor string) ([]model.CronJob, bool, error) {
	query := `SELECT id, tenant_id, webroot_id, name, schedule, command, working_directory, enabled, timeout_seconds, max_memory_mb, status, status_message, created_at, updated_at FROM cron_jobs WHERE webroot_id = $1`
	args := []any{webrootID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, cursor)
		argIdx++
	}

	query += ` ORDER BY id`
	query += fmt.Sprintf(` LIMIT $%d`, argIdx)
	args = append(args, limit+1)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list cron jobs for webroot %s: %w", webrootID, err)
	}
	defer rows.Close()

	var cronJobs []model.CronJob
	for rows.Next() {
		var c model.CronJob
		if err := rows.Scan(&c.ID, &c.TenantID, &c.WebrootID, &c.Name, &c.Schedule, &c.Command,
			&c.WorkingDirectory, &c.Enabled, &c.TimeoutSeconds, &c.MaxMemoryMB,
			&c.Status, &c.StatusMessage, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan cron job: %w", err)
		}
		cronJobs = append(cronJobs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate cron jobs: %w", err)
	}

	hasMore := len(cronJobs) > limit
	if hasMore {
		cronJobs = cronJobs[:limit]
	}
	return cronJobs, hasMore, nil
}

func (s *CronJobService) Update(ctx context.Context, cronJob *model.CronJob) error {
	_, err := s.db.Exec(ctx,
		`UPDATE cron_jobs SET schedule = $1, command = $2, working_directory = $3, timeout_seconds = $4,
		 max_memory_mb = $5, status = $6, updated_at = now() WHERE id = $7`,
		cronJob.Schedule, cronJob.Command, cronJob.WorkingDirectory, cronJob.TimeoutSeconds,
		cronJob.MaxMemoryMB, cronJob.Status, cronJob.ID,
	)
	if err != nil {
		return fmt.Errorf("update cron job %s: %w", cronJob.ID, err)
	}

	if err := signalProvision(ctx, s.tc, cronJob.TenantID, model.ProvisionTask{
		WorkflowName: "UpdateCronJobWorkflow",
		WorkflowID:   workflowID("cron-job", cronJob.Name, cronJob.ID),
		Arg:          cronJob.ID,
		ResourceType: "cron_job",
		ResourceID:   cronJob.ID,
	}); err != nil {
		return fmt.Errorf("start UpdateCronJobWorkflow: %w", err)
	}

	return nil
}

func (s *CronJobService) Delete(ctx context.Context, id string) error {
	var name string
	err := s.db.QueryRow(ctx,
		"UPDATE cron_jobs SET status = $1, updated_at = now() WHERE id = $2 RETURNING name",
		model.StatusDeleting, id,
	).Scan(&name)
	if err != nil {
		return fmt.Errorf("set cron job %s status to deleting: %w", id, err)
	}

	tenantID, err := resolveTenantIDFromCronJob(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("delete cron job: %w", err)
	}

	if err := signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteCronJobWorkflow",
		WorkflowID:   workflowID("cron-job", name, id),
		Arg:          id,
		ResourceType: "cron_job",
		ResourceID:   id,
	}); err != nil {
		return fmt.Errorf("start DeleteCronJobWorkflow: %w", err)
	}

	return nil
}

func (s *CronJobService) Enable(ctx context.Context, id string) error {
	var status, name, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, name, tenant_id FROM cron_jobs WHERE id = $1", id).Scan(&status, &name, &tenantID)
	if err != nil {
		return fmt.Errorf("get cron job status: %w", err)
	}
	if status != model.StatusActive {
		return fmt.Errorf("cron job %s is not in active state (current: %s)", id, status)
	}

	_, err = s.db.Exec(ctx,
		"UPDATE cron_jobs SET enabled = true, status = $1, updated_at = now() WHERE id = $2",
		model.StatusProvisioning, id,
	)
	if err != nil {
		return fmt.Errorf("enable cron job %s: %w", id, err)
	}

	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "EnableCronJobWorkflow",
		WorkflowID:   workflowID("cron-job", name, id),
		Arg:          id,
		ResourceType: "cron_job",
		ResourceID:   id,
	})
}

func (s *CronJobService) Disable(ctx context.Context, id string) error {
	var status, name, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, name, tenant_id FROM cron_jobs WHERE id = $1", id).Scan(&status, &name, &tenantID)
	if err != nil {
		return fmt.Errorf("get cron job status: %w", err)
	}
	if status != model.StatusActive {
		return fmt.Errorf("cron job %s is not in active state (current: %s)", id, status)
	}

	_, err = s.db.Exec(ctx,
		"UPDATE cron_jobs SET enabled = false, status = $1, updated_at = now() WHERE id = $2",
		model.StatusProvisioning, id,
	)
	if err != nil {
		return fmt.Errorf("disable cron job %s: %w", id, err)
	}

	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "DisableCronJobWorkflow",
		WorkflowID:   workflowID("cron-job", name, id),
		Arg:          id,
		ResourceType: "cron_job",
		ResourceID:   id,
	})
}

func (s *CronJobService) Retry(ctx context.Context, id string) error {
	var status, name string
	err := s.db.QueryRow(ctx, "SELECT status, name FROM cron_jobs WHERE id = $1", id).Scan(&status, &name)
	if err != nil {
		return fmt.Errorf("get cron job status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("cron job %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE cron_jobs SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set cron job %s status to provisioning: %w", id, err)
	}
	tenantID, err := resolveTenantIDFromCronJob(ctx, s.db, id)
	if err != nil {
		return fmt.Errorf("retry cron job: %w", err)
	}
	return signalProvision(ctx, s.tc, tenantID, model.ProvisionTask{
		WorkflowName: "CreateCronJobWorkflow",
		WorkflowID:   workflowID("cron-job", name, id),
		Arg:          id,
		ResourceType: "cron_job",
		ResourceID:   id,
	})
}
