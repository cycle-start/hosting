package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/edvin/hosting/internal/model"
)

func (r *Reconciler) reconcileDatabase(ctx context.Context, ds *model.DesiredState) ([]DriftEvent, error) {
	var events []DriftEvent
	fixes := 0

	for _, db := range ds.Databases {
		if fixes >= r.maxFixes {
			break
		}

		// Ensure database exists (CREATE DATABASE IF NOT EXISTS is idempotent).
		unlock := r.LockResource("database", db.Name, "")
		err := r.server.DatabaseManager().CreateDatabase(ctx, db.Name)
		unlock()
		if err != nil {
			events = append(events, DriftEvent{
				Timestamp: time.Now(), NodeID: r.nodeID, Kind: "database",
				Resource: db.Name, Action: "reported",
				Detail: fmt.Sprintf("failed to ensure database exists: %v", err),
			})
			continue
		}

		// Ensure users exist with correct grants.
		for _, u := range db.Users {
			if fixes >= r.maxFixes {
				break
			}

			unlock := r.LockResource("db_user", db.Name, u.Username)
			err := r.server.DatabaseManager().CreateUser(ctx, db.Name, u.Username, u.Password, u.Privileges)
			unlock()
			if err != nil {
				events = append(events, DriftEvent{
					Timestamp: time.Now(), NodeID: r.nodeID, Kind: "db_user",
					Resource: db.Name + "/" + u.Username, Action: "reported",
					Detail: fmt.Sprintf("failed to ensure user exists: %v", err),
				})
			}
		}
	}

	return events, nil
}
