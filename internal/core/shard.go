package core

import (
	"context"
	"encoding/json"
	"fmt"

	temporalclient "go.temporal.io/sdk/client"

	"github.com/edvin/hosting/internal/model"
)

type ShardService struct {
	db DB
	tc temporalclient.Client
}

func NewShardService(db DB, tc temporalclient.Client) *ShardService {
	return &ShardService{db: db, tc: tc}
}

func (s *ShardService) Create(ctx context.Context, shard *model.Shard) error {
	if shard.Config == nil {
		shard.Config = json.RawMessage(`{}`)
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO shards (id, cluster_id, name, role, lb_backend, config, status, status_message, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		shard.ID, shard.ClusterID, shard.Name, shard.Role, shard.LBBackend,
		shard.Config, shard.Status, shard.StatusMessage, shard.CreatedAt, shard.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create shard: %w", err)
	}
	return nil
}

func (s *ShardService) GetByID(ctx context.Context, id string) (*model.Shard, error) {
	var sh model.Shard
	err := s.db.QueryRow(ctx,
		`SELECT id, cluster_id, name, role, lb_backend, config, status, status_message, created_at, updated_at
		 FROM shards WHERE id = $1`, id,
	).Scan(&sh.ID, &sh.ClusterID, &sh.Name, &sh.Role, &sh.LBBackend,
		&sh.Config, &sh.Status, &sh.StatusMessage, &sh.CreatedAt, &sh.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get shard %s: %w", id, err)
	}
	return &sh, nil
}

func (s *ShardService) ListByCluster(ctx context.Context, clusterID string, limit int, cursor string) ([]model.Shard, bool, error) {
	query := `SELECT id, cluster_id, name, role, lb_backend, config, status, status_message, created_at, updated_at FROM shards WHERE cluster_id = $1`
	args := []any{clusterID}
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
		return nil, false, fmt.Errorf("list shards for cluster %s: %w", clusterID, err)
	}
	defer rows.Close()

	var shards []model.Shard
	for rows.Next() {
		var sh model.Shard
		if err := rows.Scan(&sh.ID, &sh.ClusterID, &sh.Name, &sh.Role, &sh.LBBackend,
			&sh.Config, &sh.Status, &sh.StatusMessage, &sh.CreatedAt, &sh.UpdatedAt); err != nil {
			return nil, false, fmt.Errorf("scan shard: %w", err)
		}
		shards = append(shards, sh)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate shards: %w", err)
	}

	hasMore := len(shards) > limit
	if hasMore {
		shards = shards[:limit]
	}
	return shards, hasMore, nil
}

func (s *ShardService) Update(ctx context.Context, shard *model.Shard) error {
	_, err := s.db.Exec(ctx,
		`UPDATE shards SET lb_backend = $1, config = $2, status = $3, status_message = $4, updated_at = now()
		 WHERE id = $5`,
		shard.LBBackend, shard.Config, shard.Status, shard.StatusMessage, shard.ID,
	)
	if err != nil {
		return fmt.Errorf("update shard %s: %w", shard.ID, err)
	}
	return nil
}

func (s *ShardService) Delete(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, "DELETE FROM shards WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete shard %s: %w", id, err)
	}
	return nil
}

func (s *ShardService) Retry(ctx context.Context, id string) error {
	var status string
	err := s.db.QueryRow(ctx, "SELECT status FROM shards WHERE id = $1", id).Scan(&status)
	if err != nil {
		return fmt.Errorf("get shard status: %w", err)
	}
	if status != model.StatusFailed {
		return fmt.Errorf("shard %s is not in failed state (current: %s)", id, status)
	}
	_, err = s.db.Exec(ctx, "UPDATE shards SET status = $1, status_message = NULL, updated_at = now() WHERE id = $2", model.StatusActive, id)
	if err != nil {
		return fmt.Errorf("reset shard %s status: %w", id, err)
	}
	return s.Converge(ctx, id)
}

func (s *ShardService) Converge(ctx context.Context, shardID string) error {
	var shardName string
	if err := s.db.QueryRow(ctx, "SELECT name FROM shards WHERE id = $1", shardID).Scan(&shardName); err != nil {
		return fmt.Errorf("get shard name for converge: %w", err)
	}

	wfID := workflowID("converge-shard", shardID)
	_, err := s.tc.ExecuteWorkflow(ctx, temporalclient.StartWorkflowOptions{
		ID:        wfID,
		TaskQueue: "hosting-tasks",
	}, "ConvergeShardWorkflow", struct {
		ShardID string `json:"shard_id"`
	}{ShardID: shardID})
	if err != nil {
		return fmt.Errorf("start ConvergeShardWorkflow: %w", err)
	}
	return nil
}
