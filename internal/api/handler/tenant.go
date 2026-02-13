package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Tenant struct {
	svc      *core.TenantService
	services *core.Services
}

func NewTenant(services *core.Services) *Tenant {
	return &Tenant{svc: services.Tenant, services: services}
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

	// Nested zone creation
	for _, zr := range req.Zones {
		now2 := time.Now()
		tenantID := tenant.ID
		zone := &model.Zone{
			ID:        platform.NewID(),
			TenantID:  &tenantID,
			Name:      zr.Name,
			RegionID:  tenant.RegionID,
			Status:    model.StatusPending,
			CreatedAt: now2,
			UpdatedAt: now2,
		}
		if err := h.services.Zone.Create(r.Context(), zone); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create zone %s: %s", zr.Name, err.Error()))
			return
		}
	}

	// Nested webroot creation
	for _, wr := range req.Webroots {
		now2 := time.Now()
		runtimeConfig := wr.RuntimeConfig
		if runtimeConfig == nil {
			runtimeConfig = json.RawMessage(`{}`)
		}
		webroot := &model.Webroot{
			ID:             platform.NewShortID(),
			TenantID:       tenant.ID,
			Name:           wr.Name,
			Runtime:        wr.Runtime,
			RuntimeVersion: wr.RuntimeVersion,
			RuntimeConfig:  runtimeConfig,
			PublicFolder:   wr.PublicFolder,
			Status:         model.StatusPending,
			CreatedAt:      now2,
			UpdatedAt:      now2,
		}
		if err := h.services.Webroot.Create(r.Context(), webroot); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create webroot %s: %s", wr.Name, err.Error()))
			return
		}
		if err := createNestedFQDNs(r.Context(), h.services, webroot.ID, wr.FQDNs); err != nil {
			response.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Nested database creation
	for _, dr := range req.Databases {
		now2 := time.Now()
		tenantID := tenant.ID
		dbShardID := dr.ShardID
		database := &model.Database{
			ID:        platform.NewID(),
			TenantID:  &tenantID,
			Name:      dr.Name,
			ShardID:   &dbShardID,
			Status:    model.StatusPending,
			CreatedAt: now2,
			UpdatedAt: now2,
		}
		if err := h.services.Database.Create(r.Context(), database); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create database %s: %s", dr.Name, err.Error()))
			return
		}
		for _, ur := range dr.Users {
			now3 := time.Now()
			user := &model.DatabaseUser{
				ID:         platform.NewID(),
				DatabaseID: database.ID,
				Username:   ur.Username,
				Password:   ur.Password,
				Privileges: ur.Privileges,
				Status:     model.StatusPending,
				CreatedAt:  now3,
				UpdatedAt:  now3,
			}
			if err := h.services.DatabaseUser.Create(r.Context(), user); err != nil {
				response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create database user %s: %s", ur.Username, err.Error()))
				return
			}
		}
	}

	// Nested valkey instance creation
	for _, vr := range req.ValkeyInstances {
		now2 := time.Now()
		tenantID := tenant.ID
		vShardID := vr.ShardID
		maxMemoryMB := vr.MaxMemoryMB
		if maxMemoryMB == 0 {
			maxMemoryMB = 64
		}
		instance := &model.ValkeyInstance{
			ID:          platform.NewID(),
			TenantID:    &tenantID,
			Name:        vr.Name,
			ShardID:     &vShardID,
			MaxMemoryMB: maxMemoryMB,
			Password:    generatePassword(),
			Status:      model.StatusPending,
			CreatedAt:   now2,
			UpdatedAt:   now2,
		}
		if err := h.services.ValkeyInstance.Create(r.Context(), instance); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create valkey instance %s: %s", vr.Name, err.Error()))
			return
		}
		for _, ur := range vr.Users {
			keyPattern := ur.KeyPattern
			if keyPattern == "" {
				keyPattern = "~*"
			}
			now3 := time.Now()
			user := &model.ValkeyUser{
				ID:               platform.NewID(),
				ValkeyInstanceID: instance.ID,
				Username:         ur.Username,
				Password:         ur.Password,
				Privileges:       ur.Privileges,
				KeyPattern:       keyPattern,
				Status:           model.StatusPending,
				CreatedAt:        now3,
				UpdatedAt:        now3,
			}
			if err := h.services.ValkeyUser.Create(r.Context(), user); err != nil {
				response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create valkey user %s: %s", ur.Username, err.Error()))
				return
			}
		}
	}

	// Nested SFTP key creation
	for _, kr := range req.SFTPKeys {
		fingerprint, err := parseSSHKey(kr.PublicKey)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create sftp key %s: invalid SSH public key: %s", kr.Name, err.Error()))
			return
		}
		now2 := time.Now()
		key := &model.SFTPKey{
			ID:          platform.NewID(),
			TenantID:    tenant.ID,
			Name:        kr.Name,
			PublicKey:   kr.PublicKey,
			Fingerprint: fingerprint,
			Status:      model.StatusPending,
			CreatedAt:   now2,
			UpdatedAt:   now2,
		}
		if err := h.services.SFTPKey.Create(r.Context(), key); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create sftp key %s: %s", kr.Name, err.Error()))
			return
		}
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

// ResourceSummary godoc
//
//	@Summary		Get resource summary for a tenant
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		200 {object} model.TenantResourceSummary
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/resource-summary [get]
func (h *Tenant) ResourceSummary(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	summary, err := h.svc.ResourceSummary(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, summary)
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
