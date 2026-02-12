package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/go-chi/chi/v5"
)

type NodeDeployment struct {
	svc *core.NodeDeploymentService
}

func NewNodeDeployment(svc *core.NodeDeploymentService) *NodeDeployment {
	return &NodeDeployment{svc: svc}
}

func (h *NodeDeployment) GetByNode(w http.ResponseWriter, r *http.Request) {
	nodeID, err := request.RequireID(chi.URLParam(r, "nodeID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	deployment, err := h.svc.GetByNodeID(r.Context(), nodeID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, deployment)
}

func (h *NodeDeployment) ListByHost(w http.ResponseWriter, r *http.Request) {
	hostID, err := request.RequireID(chi.URLParam(r, "hostID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	deployments, err := h.svc.ListByHost(r.Context(), hostID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, deployments)
}
