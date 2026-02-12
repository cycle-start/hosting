package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type HostMachine struct {
	svc *core.HostMachineService
}

func NewHostMachine(svc *core.HostMachineService) *HostMachine {
	return &HostMachine{svc: svc}
}

func (h *HostMachine) ListByCluster(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	hosts, err := h.svc.ListByCluster(r.Context(), clusterID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, hosts)
}

func (h *HostMachine) Create(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateHostMachine
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	capacity := req.Capacity
	if capacity == nil {
		capacity = json.RawMessage(`{"max_nodes": 10}`)
	}

	roles := req.Roles
	if roles == nil {
		roles = []string{}
	}

	now := time.Now()
	host := &model.HostMachine{
		ID:            platform.NewID(),
		ClusterID:     clusterID,
		Hostname:      req.Hostname,
		IPAddress:     req.IPAddress,
		DockerHost:    req.DockerHost,
		CACertPEM:     req.CACertPEM,
		ClientCertPEM: req.ClientCertPEM,
		ClientKeyPEM:  req.ClientKeyPEM,
		Capacity:      capacity,
		Roles:         roles,
		Status:        model.StatusActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := h.svc.Create(r.Context(), host); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Zero out sensitive fields before responding.
	host.CACertPEM = ""
	host.ClientCertPEM = ""
	host.ClientKeyPEM = ""

	response.WriteJSON(w, http.StatusCreated, host)
}

func (h *HostMachine) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	host, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	// Zero out sensitive fields.
	host.CACertPEM = ""
	host.ClientCertPEM = ""
	host.ClientKeyPEM = ""

	response.WriteJSON(w, http.StatusOK, host)
}

func (h *HostMachine) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateHostMachine
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	host, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Hostname != "" {
		host.Hostname = req.Hostname
	}
	if req.IPAddress != "" {
		host.IPAddress = req.IPAddress
	}
	if req.DockerHost != "" {
		host.DockerHost = req.DockerHost
	}
	if req.CACertPEM != "" {
		host.CACertPEM = req.CACertPEM
	}
	if req.ClientCertPEM != "" {
		host.ClientCertPEM = req.ClientCertPEM
	}
	if req.ClientKeyPEM != "" {
		host.ClientKeyPEM = req.ClientKeyPEM
	}
	if req.Capacity != nil {
		host.Capacity = req.Capacity
	}
	if req.Roles != nil {
		host.Roles = req.Roles
	}
	if req.Status != "" {
		host.Status = req.Status
	}

	if err := h.svc.Update(r.Context(), host); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	host.CACertPEM = ""
	host.ClientCertPEM = ""
	host.ClientKeyPEM = ""

	response.WriteJSON(w, http.StatusOK, host)
}

func (h *HostMachine) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
