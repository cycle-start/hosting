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

type NodeProfile struct {
	svc *core.NodeProfileService
}

func NewNodeProfile(svc *core.NodeProfileService) *NodeProfile {
	return &NodeProfile{svc: svc}
}

func (h *NodeProfile) List(w http.ResponseWriter, r *http.Request) {
	profiles, err := h.svc.List(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, profiles)
}

func (h *NodeProfile) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateNodeProfile
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	profile := &model.NodeProfile{
		ID:          platform.NewID(),
		Name:        req.Name,
		Role:        req.Role,
		Image:       req.Image,
		Env:         defaultJSON(req.Env, "{}"),
		Volumes:     defaultJSON(req.Volumes, "[]"),
		Ports:       defaultJSON(req.Ports, "[]"),
		Resources:   defaultJSON(req.Resources, `{"memory_mb": 2048, "cpu_shares": 1024}`),
		HealthCheck: defaultJSON(req.HealthCheck, "{}"),
		NetworkMode: "bridge",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if req.Privileged != nil {
		profile.Privileged = *req.Privileged
	}
	if req.NetworkMode != "" {
		profile.NetworkMode = req.NetworkMode
	}

	if err := h.svc.Create(r.Context(), profile); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, profile)
}

func (h *NodeProfile) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	profile, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

func (h *NodeProfile) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateNodeProfile
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	profile, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Name != "" {
		profile.Name = req.Name
	}
	if req.Role != "" {
		profile.Role = req.Role
	}
	if req.Image != "" {
		profile.Image = req.Image
	}
	if req.Env != nil {
		profile.Env = req.Env
	}
	if req.Volumes != nil {
		profile.Volumes = req.Volumes
	}
	if req.Ports != nil {
		profile.Ports = req.Ports
	}
	if req.Resources != nil {
		profile.Resources = req.Resources
	}
	if req.HealthCheck != nil {
		profile.HealthCheck = req.HealthCheck
	}
	if req.Privileged != nil {
		profile.Privileged = *req.Privileged
	}
	if req.NetworkMode != "" {
		profile.NetworkMode = req.NetworkMode
	}

	if err := h.svc.Update(r.Context(), profile); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, profile)
}

func (h *NodeProfile) Delete(w http.ResponseWriter, r *http.Request) {
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

func defaultJSON(v json.RawMessage, fallback string) json.RawMessage {
	if v == nil {
		return json.RawMessage(fallback)
	}
	return v
}
