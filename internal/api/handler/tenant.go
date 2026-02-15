package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	mw "github.com/edvin/hosting/internal/api/middleware"
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

// checkTenantBrandAccess fetches a tenant and verifies brand access for the caller.
func (h *Tenant) checkTenantBrandAccess(w http.ResponseWriter, r *http.Request, id string) bool {
	tenant, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return false
	}
	if !mw.HasBrandAccess(mw.GetIdentity(r.Context()), tenant.BrandID) {
		response.WriteError(w, http.StatusForbidden, "no access to this brand")
		return false
	}
	return true
}

// List godoc
//
//	@Summary		List tenants
//	@Description	Returns a paginated list of tenants with optional search, status filtering, and sorting. Includes computed region, cluster, and shard names.
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
	params.BrandIDs = mw.BrandIDs(r.Context())

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
//	@Description	Creates a new tenant with a generated short ID and UID. Supports nested creation of zones, webroots, databases, valkey instances, S3 buckets, and SSH keys in one request. Validates that the target cluster is in the brand's allowed cluster list. Async — returns 202 and triggers Temporal provisioning workflows for each resource.
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

	if !mw.HasBrandAccess(mw.GetIdentity(r.Context()), req.BrandID) {
		response.WriteError(w, http.StatusForbidden, "no access to this brand")
		return
	}

	// Validate cluster is in brand's allowed list (if any).
	allowedClusters, err := h.services.Brand.ListClusters(r.Context(), req.BrandID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(allowedClusters) > 0 {
		found := false
		for _, c := range allowedClusters {
			if c == req.ClusterID {
				found = true
				break
			}
		}
		if !found {
			response.WriteError(w, http.StatusBadRequest, fmt.Sprintf("cluster %s is not allowed for brand %s", req.ClusterID, req.BrandID))
			return
		}
	}

	now := time.Now()
	shardID := req.ShardID
	tenant := &model.Tenant{
		ID:        platform.NewID(),
		Name:      platform.NewName("t_"),
		BrandID:   req.BrandID,
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
	if req.SSHEnabled != nil {
		tenant.SSHEnabled = *req.SSHEnabled
	}
	if req.DiskQuotaBytes != nil {
		tenant.DiskQuotaBytes = *req.DiskQuotaBytes
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
			ID:             platform.NewID(),
			TenantID:       tenant.ID,
			Name:           platform.NewName("web_"),
			Runtime:        wr.Runtime,
			RuntimeVersion: wr.RuntimeVersion,
			RuntimeConfig:  runtimeConfig,
			PublicFolder:   wr.PublicFolder,
			Status:         model.StatusPending,
			CreatedAt:      now2,
			UpdatedAt:      now2,
		}
		if err := h.services.Webroot.Create(r.Context(), webroot); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create webroot: %s", err.Error()))
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
		dbName := platform.NewName("db_")
		database := &model.Database{
			ID:        platform.NewID(),
			TenantID:  &tenantID,
			Name:      dbName,
			ShardID:   &dbShardID,
			Status:    model.StatusPending,
			CreatedAt: now2,
			UpdatedAt: now2,
		}
		if err := h.services.Database.Create(r.Context(), database); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create database %s: %s", dbName, err.Error()))
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
			Name:        platform.NewName("kv_"),
			ShardID:     &vShardID,
			MaxMemoryMB: maxMemoryMB,
			Password:    generatePassword(),
			Status:      model.StatusPending,
			CreatedAt:   now2,
			UpdatedAt:   now2,
		}
		if err := h.services.ValkeyInstance.Create(r.Context(), instance); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create valkey instance: %s", err.Error()))
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

	// Nested S3 bucket creation
	for _, br := range req.S3Buckets {
		now2 := time.Now()
		tenantID := tenant.ID
		s3ShardID := br.ShardID
		bucket := &model.S3Bucket{
			ID:        platform.NewID(),
			TenantID:  &tenantID,
			Name:      platform.NewName("s3_"),
			ShardID:   &s3ShardID,
			Status:    model.StatusPending,
			CreatedAt: now2,
			UpdatedAt: now2,
		}
		if br.Public != nil && *br.Public {
			bucket.Public = true
		}
		if br.QuotaBytes != nil {
			bucket.QuotaBytes = *br.QuotaBytes
		}
		if err := h.services.S3Bucket.Create(r.Context(), bucket); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create s3 bucket: %s", err.Error()))
			return
		}
	}

	// Nested SSH key creation
	for _, kr := range req.SSHKeys {
		fingerprint, err := parseSSHKey(kr.PublicKey)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create SSH key %s: invalid SSH public key: %s", kr.Name, err.Error()))
			return
		}
		now2 := time.Now()
		key := &model.SSHKey{
			ID:          platform.NewID(),
			TenantID:    tenant.ID,
			Name:        kr.Name,
			PublicKey:   kr.PublicKey,
			Fingerprint: fingerprint,
			Status:      model.StatusPending,
			CreatedAt:   now2,
			UpdatedAt:   now2,
		}
		if err := h.services.SSHKey.Create(r.Context(), key); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create SSH key %s: %s", kr.Name, err.Error()))
			return
		}
	}

	response.WriteJSON(w, http.StatusAccepted, tenant)
}

// Get godoc
//
//	@Summary		Get a tenant
//	@Description	Returns a single tenant by ID, including computed region, cluster, and shard names.
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

	if !mw.HasBrandAccess(mw.GetIdentity(r.Context()), tenant.BrandID) {
		response.WriteError(w, http.StatusForbidden, "no access to this brand")
		return
	}

	response.WriteJSON(w, http.StatusOK, tenant)
}

// Update godoc
//
//	@Summary		Update a tenant
//	@Description	Partial update of a tenant — currently supports toggling sftp_enabled and ssh_enabled. Async — returns 202 and triggers re-convergence of the tenant's web shard.
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

	if !mw.HasBrandAccess(mw.GetIdentity(r.Context()), tenant.BrandID) {
		response.WriteError(w, http.StatusForbidden, "no access to this brand")
		return
	}

	if req.SFTPEnabled != nil {
		tenant.SFTPEnabled = *req.SFTPEnabled
	}
	if req.SSHEnabled != nil {
		tenant.SSHEnabled = *req.SSHEnabled
	}
	if req.DiskQuotaBytes != nil {
		tenant.DiskQuotaBytes = *req.DiskQuotaBytes
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
//	@Description	Marks a tenant for deletion. Async — returns 202 and triggers a Temporal workflow that cascades deletion to all child resources (webroots, FQDNs, databases, etc.).
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

	if !h.checkTenantBrandAccess(w, r, id) {
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
//	@Description	Suspends a tenant, disabling all web traffic and services. Cascades suspension to all child resources. Async — returns 202 and triggers a workflow that converges shard configuration.
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Param			body body object true "Suspend request" example({"reason": "abuse"})
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

	var req struct {
		Reason string `json:"reason" validate:"required"`
	}
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Reason == "" {
		response.WriteError(w, http.StatusBadRequest, "reason is required")
		return
	}

	if !h.checkTenantBrandAccess(w, r, id) {
		return
	}

	if err := h.svc.Suspend(r.Context(), id, req.Reason); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Unsuspend godoc
//
//	@Summary		Unsuspend a tenant
//	@Description	Re-enables a suspended tenant, restoring web traffic and services. Async — returns 202 and triggers a workflow that restores shard configuration.
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

	if !h.checkTenantBrandAccess(w, r, id) {
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
//	@Description	Returns a synchronous breakdown of all child resources grouped by type and status, including active, pending, and failed counts for each resource type.
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

	if !h.checkTenantBrandAccess(w, r, id) {
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
//	@Description	Moves a tenant to a different web shard. Optionally migrates associated zones and FQDNs to the target shard. Async — returns 202 and triggers a multi-step Temporal migration workflow.
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

	if !h.checkTenantBrandAccess(w, r, id) {
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

// Retry godoc
//
//	@Summary		Retry a failed tenant
//	@Description	Re-triggers the provisioning workflow for a tenant in failed state. Async — returns 202 and starts a new Temporal workflow.
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/retry [post]
func (h *Tenant) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.checkTenantBrandAccess(w, r, id) {
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

// RetryFailed godoc
//
//	@Summary		Retry all failed resources for a tenant
//	@Description	Finds all child resources in failed state and re-triggers their provisioning workflows. Async — returns 202 with the count of resources being retried.
//	@Tags			Tenants
//	@Security		ApiKeyAuth
//	@Param			id path string true "Tenant ID"
//	@Success		202 {object} map[string]any
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{id}/retry-failed [post]
func (h *Tenant) RetryFailed(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if !h.checkTenantBrandAccess(w, r, id) {
		return
	}
	count, err := h.svc.RetryFailed(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusAccepted, map[string]any{"status": "retrying", "count": count})
}
