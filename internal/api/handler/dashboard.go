package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
)

type Dashboard struct {
	svc *core.DashboardService
}

func NewDashboard(svc *core.DashboardService) *Dashboard {
	return &Dashboard{svc: svc}
}

// Stats godoc
//
//	@Summary		Get dashboard statistics
//	@Description	Returns aggregate platform statistics including counts of regions, clusters, shards, nodes, tenants (broken down by status), databases, zones, valkey instances, and FQDNs. Also includes per-shard tenant distribution and per-cluster node counts. Synchronous.
//	@Tags			Dashboard
//	@Security		ApiKeyAuth
//	@Success		200	{object}	map[string]any
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/dashboard/stats [get]
func (h *Dashboard) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.Stats(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, stats)
}
