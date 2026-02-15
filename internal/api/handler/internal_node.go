package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
)

type InternalNode struct {
	desiredStateSvc *core.DesiredStateService
	healthSvc       *core.NodeHealthService
}

func NewInternalNode(ds *core.DesiredStateService, hs *core.NodeHealthService) *InternalNode {
	return &InternalNode{desiredStateSvc: ds, healthSvc: hs}
}

// GetDesiredState returns the full desired state for a node.
func (h *InternalNode) GetDesiredState(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing node ID")
		return
	}

	ds, err := h.desiredStateSvc.GetForNode(r.Context(), nodeID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch desired state")
		return
	}

	response.WriteJSON(w, http.StatusOK, ds)
}

// ReportHealth accepts a health report from a node agent.
func (h *InternalNode) ReportHealth(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing node ID")
		return
	}

	var health model.NodeHealth
	if err := json.NewDecoder(r.Body).Decode(&health); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	health.NodeID = nodeID

	if err := h.healthSvc.UpsertHealth(r.Context(), &health); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to store health report")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetHealth returns the latest health report for a node.
func (h *InternalNode) GetHealth(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing node ID")
		return
	}

	health, err := h.healthSvc.GetHealth(r.Context(), nodeID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "no health report found")
		return
	}

	response.WriteJSON(w, http.StatusOK, health)
}

// ReportDriftEvents accepts drift events from a node agent.
func (h *InternalNode) ReportDriftEvents(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing node ID")
		return
	}

	var req struct {
		Events []model.DriftEvent `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Set node ID on all events
	for i := range req.Events {
		req.Events[i].NodeID = nodeID
	}

	if err := h.healthSvc.CreateDriftEvents(r.Context(), req.Events); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to store drift events")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ListDriftEvents returns recent drift events for a node.
func (h *InternalNode) ListDriftEvents(w http.ResponseWriter, r *http.Request) {
	nodeID := chi.URLParam(r, "nodeID")
	if nodeID == "" {
		response.WriteError(w, http.StatusBadRequest, "missing node ID")
		return
	}

	events, err := h.healthSvc.ListDriftEvents(r.Context(), nodeID, 100)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch drift events")
		return
	}
	if events == nil {
		events = []model.DriftEvent{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"items":    events,
		"has_more": false,
	})
}
