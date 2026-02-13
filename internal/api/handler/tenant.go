package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Tenant struct {
	svc *core.TenantService
}

func NewTenant(svc *core.TenantService) *Tenant {
	return &Tenant{svc: svc}
}

// List godoc
//
//	@Summary		List tenants
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			search query string false "Search query"
//	@Param			status query string false "Filter by status"
//	@Param			sort query string false "Sort field" default(created_at)
//	@Param			order query string false "Sort order (asc/desc)" default(asc)
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.Tenant}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants [get]
func (h *Tenant) List(w http.ResponseWriter, r *http.Request) {
	params := request.ParseListParams(r, "created_at")

	tenants, hasMore, err := h.svc.List(r.Context(), params)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(tenants) > 0 {
		nextCursor = tenants[len(tenants)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, tenants, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a tenant
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			body body request.CreateTenant true "Tenant details"
//	@Success		202 {object} model.Tenant
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants [post]
func (h *Tenant) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateTenant
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	shardID := req.ShardID
	tenant := &model.Tenant{
		ID:        platform.NewShortID(),
		Name:      req.Name,
		RegionID:  req.RegionID,
		ClusterID: req.ClusterID,
		ShardID:   &shardID,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if req.SFTPEnabled != nil {
		tenant.SFTPEnabled = *req.SFTPEnabled
	}

	if err := h.svc.Create(r.Context(), tenant); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, tenant)
}

// Get godoc
//
//	@Summary		Get a tenant
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		200 {object} model.Tenant
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/tenants/{id} [get]
func (h *Tenant) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenant, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, tenant)
}

// Update godoc
//
//	@Summary		Update a tenant
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Param			body body request.UpdateTenant true "Tenant updates"
//	@Success		202 {object} model.Tenant
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id} [put]
func (h *Tenant) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateTenant
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	tenant, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.SFTPEnabled != nil {
		tenant.SFTPEnabled = *req.SFTPEnabled
	}

	if err := h.svc.Update(r.Context(), tenant); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, tenant)
}

// Delete godoc
//
//	@Summary		Delete a tenant
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id} [delete]
func (h *Tenant) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Suspend godoc
//
//	@Summary		Suspend a tenant
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/suspend [post]
func (h *Tenant) Suspend(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Suspend(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Unsuspend godoc
//
//	@Summary		Unsuspend a tenant
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/unsuspend [post]
func (h *Tenant) Unsuspend(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Unsuspend(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Migrate godoc
//
//	@Summary		Migrate a tenant to another shard
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Param			body body request.MigrateTenant true "Migration details"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/migrate [post]
func (h *Tenant) Migrate(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.MigrateTenant
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Migrate(r.Context(), id, req.TargetShardID, req.MigrateZones, req.MigrateFQDNs); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
