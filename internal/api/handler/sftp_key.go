package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

// SFTPKey handles SFTP key management endpoints.
type SFTPKey struct {
	svc       *core.SFTPKeyService
	tenantSvc *core.TenantService
}

// NewSFTPKey creates a new SFTPKey handler.
func NewSFTPKey(svc *core.SFTPKeyService, tenantSvc *core.TenantService) *SFTPKey {
	return &SFTPKey{svc: svc, tenantSvc: tenantSvc}
}

// ListByTenant godoc
//
//	@Summary		List SFTP keys for a tenant
//	@Description	Returns a paginated list of SFTP public keys associated with the specified tenant.
//	@Tags			SFTP Keys
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.SFTPKey}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/sftp-keys [get]
func (h *SFTPKey) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	pg := request.ParsePagination(r)

	keys, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(keys) > 0 {
		nextCursor = keys[len(keys)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, keys, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an SFTP key
//	@Description	Adds an SSH public key for SFTP access to a tenant's files. The key fingerprint is computed server-side. Async — returns 202 and triggers a workflow to deploy the key to all nodes in the shard.
//	@Tags			SFTP Keys
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			body body request.CreateSFTPKey true "SFTP key details"
//	@Success		202 {object} model.SFTPKey
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/sftp-keys [post]
func (h *SFTPKey) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateSFTPKey
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Parse the public key and compute its fingerprint.
	fingerprint, err := parseSSHKey(req.PublicKey)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid SSH public key: "+err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, tenantID) {
		return
	}

	now := time.Now()
	key := &model.SFTPKey{
		ID:          platform.NewID(),
		TenantID:    tenantID,
		Name:        req.Name,
		PublicKey:   req.PublicKey,
		Fingerprint: fingerprint,
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.svc.Create(r.Context(), key); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, key)
}

// Get godoc
//
//	@Summary		Get an SFTP key
//	@Description	Returns a single SFTP key by ID, including its public key and computed fingerprint.
//	@Tags			SFTP Keys
//	@Security		ApiKeyAuth
//	@Param			id path string true "SFTP key ID"
//	@Success		200 {object} model.SFTPKey
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/sftp-keys/{id} [get]
func (h *SFTPKey) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	key, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.tenantSvc, key.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, key)
}

// Delete godoc
//
//	@Summary		Delete an SFTP key
//	@Description	Removes an SFTP key from all nodes in the tenant's shard. Async — returns 202 and triggers a Temporal workflow.
//	@Tags			SFTP Keys
//	@Security		ApiKeyAuth
//	@Param			id path string true "SFTP key ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/sftp-keys/{id} [delete]
func (h *SFTPKey) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	key, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, key.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Retry godoc
//
//	@Summary		Retry a failed SFTP key
//	@Description	Re-triggers the deployment workflow for an SFTP key in failed state. Async — returns 202 and starts a new Temporal workflow.
//	@Tags			SFTP Keys
//	@Security		ApiKeyAuth
//	@Param			id path string true "SFTP key ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/sftp-keys/{id}/retry [post]
func (h *SFTPKey) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	key, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.tenantSvc, key.TenantID) {
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
