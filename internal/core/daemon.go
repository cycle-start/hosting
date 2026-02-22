package core

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/edvin/hosting/internal/model"
	temporalclient "go.temporal.io/sdk/client"
)

type DaemonService struct {
	db DB
	tc temporalclient.Client
}

func NewDaemonService(db DB, tc temporalclient.Client) *DaemonService {
	return &DaemonService{db: db, tc: tc}
}

func (s *DaemonService) Create(ctx context.Context, daemon *model.Daemon) error {
	// Assign node_id via least-loaded round-robin across active shard nodes.
	var shardID *string
	err := s.db.QueryRow(ctx, "SELECT shard_id FROM tenants WHERE id = $1", daemon.TenantID).Scan(&shardID)
	if err != nil {
		return fmt.Errorf("look up tenant shard: %w", err)
	}

	if shardID != nil {
		var nodeID string
		err = s.db.QueryRow(ctx,
			`SELECT n.id FROM nodes n
			 JOIN node_shard_assignments nsa ON nsa.node_id = n.id
			 LEFT JOIN daemons d ON d.node_id = n.id
			 WHERE nsa.shard_id = $1 AND n.status = 'active'
			 GROUP BY n.id
			 ORDER BY COUNT(d.id) ASC, n.id ASC
			 LIMIT 1`, *shardID,
		).Scan(&nodeID)
		if err != nil {
			return fmt.Errorf("find least-loaded node in shard %s: %w", *shardID, err)
		}
		daemon.NodeID = &nodeID
	}

	_, err = s.db.Exec(ctx,
		`INSERT INTO daemons (id, tenant_id, node_id, webroot_id, name, command, proxy_path, proxy_port, num_procs, stop_signal, stop_wait_secs, max_memory_mb, enabled, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
		daemon.ID, daemon.TenantID, daemon.NodeID, daemon.WebrootID, daemon.Name, daemon.Command,
		daemon.ProxyPath, daemon.ProxyPort, daemon.NumProcs, daemon.StopSignal,
		daemon.StopWaitSecs, daemon.MaxMemoryMB,
		daemon.Enabled, daemon.Status, daemon.CreatedAt, daemon.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert daemon: %w", err)
	}

	if err := signalProvision(ctx, s.tc, s.db, daemon.TenantID, model.ProvisionTask{
		WorkflowName: "CreateDaemonWorkflow",
		WorkflowID:   fmt.Sprintf("create-daemon-%s", daemon.ID),
		Arg:          daemon.ID,
	}); err != nil {
		return fmt.Errorf("signal CreateDaemonWorkflow: %w", err)
	}

	return nil
}

const daemonColumns = `id, tenant_id, node_id, webroot_id, name, command, proxy_path, proxy_port, num_procs, stop_signal, stop_wait_secs, max_memory_mb, enabled, status, status_message, created_at, updated_at`

func scanDaemon(row interface{ Scan(dest ...any) error }) (model.Daemon, error) {
	var d model.Daemon
	err := row.Scan(&d.ID, &d.TenantID, &d.NodeID, &d.WebrootID, &d.Name, &d.Command,
		&d.ProxyPath, &d.ProxyPort, &d.NumProcs, &d.StopSignal,
		&d.StopWaitSecs, &d.MaxMemoryMB,
		&d.Enabled, &d.Status, &d.StatusMessage, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return d, err
	}
	return d, nil
}

func (s *DaemonService) GetByID(ctx context.Context, id string) (*model.Daemon, error) {
	row := s.db.QueryRow(ctx,
		`SELECT `+daemonColumns+` FROM daemons WHERE id = $1`, id,
	)
	d, err := scanDaemon(row)
	if err != nil {
		return nil, fmt.Errorf("get daemon %s: %w", id, err)
	}
	return &d, nil
}

func (s *DaemonService) ListByWebroot(ctx context.Context, webrootID string, limit int, cursor string) ([]model.Daemon, bool, error) {
	query := `SELECT ` + daemonColumns + ` FROM daemons WHERE webroot_id = $1`
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
		return nil, false, fmt.Errorf("list daemons for webroot %s: %w", webrootID, err)
	}
	defer rows.Close()

	var daemons []model.Daemon
	for rows.Next() {
		d, err := scanDaemon(rows)
		if err != nil {
			return nil, false, fmt.Errorf("scan daemon: %w", err)
		}
		daemons = append(daemons, d)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate daemons: %w", err)
	}

	hasMore := len(daemons) > limit
	if hasMore {
		daemons = daemons[:limit]
	}
	return daemons, hasMore, nil
}

func (s *DaemonService) Update(ctx context.Context, daemon *model.Daemon) error {
	_, err := s.db.Exec(ctx,
		`UPDATE daemons SET command = $1, proxy_path = $2, proxy_port = $3, num_procs = $4,
		 stop_signal = $5, stop_wait_secs = $6, max_memory_mb = $7,
		 status = $8, updated_at = now() WHERE id = $9`,
		daemon.Command, daemon.ProxyPath, daemon.ProxyPort, daemon.NumProcs,
		daemon.StopSignal, daemon.StopWaitSecs, daemon.MaxMemoryMB,
		daemon.Status, daemon.ID,
	)
	if err != nil {
		return fmt.Errorf("update daemon %s: %w", daemon.ID, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, daemon.TenantID, model.ProvisionTask{
		WorkflowName: "UpdateDaemonWorkflow",
		WorkflowID:   workflowID("daemon", daemon.Name, daemon.ID),
		Arg:          daemon.ID,
	}); err != nil {
		return fmt.Errorf("signal UpdateDaemonWorkflow: %w", err)
	}

	return nil
}

func (s *DaemonService) Delete(ctx context.Context, id string) error {
	var name, tenantID string
	err := s.db.QueryRow(ctx,
		"UPDATE daemons SET status = $1, updated_at = now() WHERE id = $2 RETURNING name, tenant_id",
		model.StatusDeleting, id,
	).Scan(&name, &tenantID)
	if err != nil {
		return fmt.Errorf("set daemon %s status to deleting: %w", id, err)
	}

	if err := signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DeleteDaemonWorkflow",
		WorkflowID:   workflowID("daemon", name, id),
		Arg:          id,
	}); err != nil {
		return fmt.Errorf("signal DeleteDaemonWorkflow: %w", err)
	}

	return nil
}

func (s *DaemonService) Enable(ctx context.Context, id string) error {
	var status, name, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, name, tenant_id FROM daemons WHERE id = $1", id).Scan(&status, &name, &tenantID)
	if err != nil {
		return fmt.Errorf("get daemon status: %w", err)
	}
	if status != model.StatusActive {
		return fmt.Errorf("daemon %s is not in active state (current: %s)", id, status)
	}

	_, err = s.db.Exec(ctx,
		"UPDATE daemons SET enabled = true, status = $1, status_message = NULL, updated_at = now() WHERE id = $2",
		model.StatusProvisioning, id,
	)
	if err != nil {
		return fmt.Errorf("enable daemon %s: %w", id, err)
	}

	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "EnableDaemonWorkflow",
		WorkflowID:   workflowID("daemon", name, id),
		Arg:          id,
	})
}

func (s *DaemonService) Disable(ctx context.Context, id string) error {
	var status, name, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, name, tenant_id FROM daemons WHERE id = $1", id).Scan(&status, &name, &tenantID)
	if err != nil {
		return fmt.Errorf("get daemon status: %w", err)
	}
	if status != model.StatusActive {
		return fmt.Errorf("daemon %s is not in active state (current: %s)", id, status)
	}

	_, err = s.db.Exec(ctx,
		"UPDATE daemons SET enabled = false, status = $1, updated_at = now() WHERE id = $2",
		model.StatusProvisioning, id,
	)
	if err != nil {
		return fmt.Errorf("disable daemon %s: %w", id, err)
	}

	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "DisableDaemonWorkflow",
		WorkflowID:   workflowID("daemon", name, id),
		Arg:          id,
	})
}

func (s *DaemonService) Retry(ctx context.Context, id string) error {
	var status, name, tenantID string
	err := s.db.QueryRow(ctx, "SELECT status, name, tenant_id FROM daemons WHERE id = $1", id).Scan(&status, &name, &tenantID)
	if err != nil {
		return fmt.Errorf("get daemon status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("daemon %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE daemons SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusProvisioning, id)
	if err != nil {
		return fmt.Errorf("set daemon %s status to provisioning: %w", id, err)
	}
	return signalProvision(ctx, s.tc, s.db, tenantID, model.ProvisionTask{
		WorkflowName: "CreateDaemonWorkflow",
		WorkflowID:   workflowID("daemon", name, id),
		Arg:          id,
	})
}

// ComputeDaemonPort derives a deterministic port from names using FNV hash.
// Ports are mapped into the range 10000-19999.
func ComputeDaemonPort(tenantName, webrootName, daemonName string) int {
	h := fnv.New32a()
	h.Write([]byte(tenantName + "/" + webrootName + "/" + daemonName))
	return 10000 + int(h.Sum32()%10000)
}
