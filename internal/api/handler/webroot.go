package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/agent/runtime"
	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Webroot struct {
	svc      *core.WebrootService
	services *core.Services
}

func NewWebroot(services *core.Services) *Webroot {
	return &Webroot{svc: services.Webroot, services: services}
}

// ListByTenant godoc
//
//	@Summary		List webroots for a tenant
//	@Description	Returns a paginated list of webroots belonging to the specified tenant.
//	@Tags			Webroots
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.Webroot}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/webroots [get]
func (h *Webroot) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, tenantID) {
		return
	}

	pg := request.ParsePagination(r)

	webroots, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(webroots) > 0 {
		nextCursor = webroots[len(webroots)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, webroots, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a webroot
//	@Description	Creates a webroot (website document root) for a tenant. Requires name, runtime (php/node/python/ruby/static), and runtime version. Supports nested FQDN creation. Async — returns 202 and triggers a Temporal workflow.
//	@Tags			Webroots
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			body body request.CreateWebroot true "Webroot details"
//	@Success		202 {object} model.Webroot
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/webroots [post]
func (h *Webroot) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateWebroot
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, tenantID) {
		return
	}

	now := time.Now()
	runtimeConfig := req.RuntimeConfig
	if runtimeConfig == nil {
		runtimeConfig = json.RawMessage(`{}`)
	}

	// Validate PHP runtime_config if applicable.
	if req.Runtime == "php" {
		if err := runtime.ValidatePHPRuntimeConfig(runtimeConfig); err != nil {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	envFileName := req.EnvFileName
	if envFileName == "" {
		envFileName = ".env.hosting"
	}
	serviceHostnameEnabled := true
	if req.ServiceHostnameEnabled != nil {
		serviceHostnameEnabled = *req.ServiceHostnameEnabled
	}

	webroot := &model.Webroot{
		ID:                     platform.NewID(),
		TenantID:               tenantID,
		Name:                   platform.NewName("w"),
		Runtime:                req.Runtime,
		RuntimeVersion:         req.RuntimeVersion,
		RuntimeConfig:          runtimeConfig,
		PublicFolder:            req.PublicFolder,
		EnvFileName:             envFileName,
		ServiceHostnameEnabled: serviceHostnameEnabled,
		Status:                 model.StatusPending,
		CreatedAt:              now,
		UpdatedAt:              now,
	}

	if err := h.svc.Create(r.Context(), webroot); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	// Nested FQDN creation
	if err := createNestedFQDNs(r.Context(), h.services, webroot.ID, tenantID, req.FQDNs); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, webroot)
}

// Get godoc
//
//	@Summary		Get a webroot
//	@Description	Returns a single webroot by ID, including runtime configuration details.
//	@Tags			Webroots
//	@Security		ApiKeyAuth
//	@Param			id path string true "Webroot ID"
//	@Success		200 {object} model.Webroot
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/webroots/{id} [get]
func (h *Webroot) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, webroot)
}

// Update godoc
//
//	@Summary		Update a webroot
//	@Description	Partial update of a webroot — supports changing runtime, version, runtime config, or public folder. Async — returns 202 and triggers re-convergence of the web server configuration.
//	@Tags			Webroots
//	@Security		ApiKeyAuth
//	@Param			id path string true "Webroot ID"
//	@Param			body body request.UpdateWebroot true "Webroot updates"
//	@Success		202 {object} model.Webroot
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/webroots/{id} [put]
func (h *Webroot) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateWebroot
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	if req.Runtime != "" {
		webroot.Runtime = req.Runtime
	}
	if req.RuntimeVersion != "" {
		webroot.RuntimeVersion = req.RuntimeVersion
	}
	if req.RuntimeConfig != nil {
		webroot.RuntimeConfig = req.RuntimeConfig
	}
	if req.PublicFolder != nil {
		webroot.PublicFolder = *req.PublicFolder
	}
	if req.EnvFileName != nil {
		webroot.EnvFileName = *req.EnvFileName
	}
	if req.ServiceHostnameEnabled != nil {
		webroot.ServiceHostnameEnabled = *req.ServiceHostnameEnabled
	}

	// Validate PHP runtime_config after merging.
	effectiveRuntime := webroot.Runtime
	if effectiveRuntime == "php" {
		if err := runtime.ValidatePHPRuntimeConfig(webroot.RuntimeConfig); err != nil {
			response.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	if err := h.svc.Update(r.Context(), webroot); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, webroot)
}

// Delete godoc
//
//	@Summary		Delete a webroot
//	@Description	Deletes a webroot and cascades deletion to all associated FQDNs. Async — returns 202 and triggers a Temporal workflow.
//	@Tags			Webroots
//	@Security		ApiKeyAuth
//	@Param			id path string true "Webroot ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/webroots/{id} [delete]
func (h *Webroot) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Retry godoc
//
//	@Summary		Retry a failed webroot
//	@Description	Re-triggers the provisioning workflow for a webroot in failed state. Async — returns 202 and starts a new Temporal workflow.
//	@Tags			Webroots
//	@Security		ApiKeyAuth
//	@Param			id path string true "Webroot ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/webroots/{id}/retry [post]
func (h *Webroot) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	webroot, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
