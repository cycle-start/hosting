package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/go-chi/chi/v5"
)

type WebrootEnvVar struct {
	svc      *core.WebrootEnvVarService
	services *core.Services
}

func NewWebrootEnvVar(services *core.Services) *WebrootEnvVar {
	return &WebrootEnvVar{svc: services.WebrootEnvVar, services: services}
}

// List returns all env vars for a webroot. Secret values are redacted to "***".
func (h *WebrootEnvVar) List(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.services.Webroot.GetByID(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	vars, err := h.svc.List(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type envVarResponse struct {
		Name     string `json:"name"`
		Value    string `json:"value"`
		IsSecret bool   `json:"is_secret"`
	}

	items := make([]envVarResponse, len(vars))
	for i, v := range vars {
		items[i] = envVarResponse{Name: v.Name, Value: v.Value, IsSecret: v.IsSecret}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Set replaces all env vars for a webroot (bulk PUT).
func (h *WebrootEnvVar) Set(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.SetWebrootEnvVars
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := req.Validate(); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.services.Webroot.GetByID(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	vars := make([]model.WebrootEnvVar, len(req.Vars))
	for i, v := range req.Vars {
		vars[i] = model.WebrootEnvVar{
			WebrootID: webrootID,
			Name:      v.Name,
			Value:     v.Value,
			IsSecret:  v.Secret,
		}
	}

	if err := h.svc.BulkSet(r.Context(), webrootID, vars); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Delete removes a single env var by name.
func (h *WebrootEnvVar) Delete(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		response.WriteError(w, http.StatusBadRequest, "missing env var name")
		return
	}

	webroot, err := h.services.Webroot.GetByID(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	if err := h.svc.DeleteByName(r.Context(), webrootID, name); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
