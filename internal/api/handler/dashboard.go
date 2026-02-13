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

func (h *Dashboard) Stats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.Stats(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, stats)
}
