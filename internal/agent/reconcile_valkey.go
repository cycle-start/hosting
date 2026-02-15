package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/model"
)

func (r *Reconciler) reconcileValkey(ctx context.Context, ds *model.DesiredState) ([]DriftEvent, error) {
	var events []DriftEvent
	fixes := 0

	for _, vi := range ds.ValkeyInstances {
		if fixes >= r.maxFixes {
			break
		}

		// Ensure instance exists (CreateInstance is idempotent).
		unlock := r.LockResource("valkey", vi.Name, "")
		err := r.server.ValkeyManager().CreateInstance(ctx, vi.Name, vi.Port, vi.Password, vi.MaxMemoryMB)
		unlock()
		if err != nil {
			events = append(events, DriftEvent{
				Timestamp: time.Now(), NodeID: r.nodeID, Kind: "valkey_instance",
				Resource: vi.Name, Action: "reported",
				Detail: fmt.Sprintf("failed to ensure instance exists: %v", err),
			})
			continue
		}

		// Ensure users.
		for _, u := range vi.Users {
			if fixes >= r.maxFixes {
				break
			}

			// Parse privileges from comma-separated string to slice.
			var privileges []string
			if u.Privileges != "" {
				for _, p := range strings.Split(u.Privileges, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						privileges = append(privileges, p)
					}
				}
			}

			unlock := r.LockResource("valkey_user", vi.Name, u.Username)
			err := r.server.ValkeyManager().CreateUser(ctx, vi.Name, vi.Port, u.Username, u.Password, privileges, u.KeyPattern)
			unlock()
			if err != nil {
				events = append(events, DriftEvent{
					Timestamp: time.Now(), NodeID: r.nodeID, Kind: "valkey_user",
					Resource: vi.Name + "/" + u.Username, Action: "reported",
					Detail: fmt.Sprintf("failed to ensure user exists: %v", err),
				})
			}
		}
	}

	return events, nil
}
