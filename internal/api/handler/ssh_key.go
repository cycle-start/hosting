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

// SSHKey handles SSH key management endpoints.
type SSHKey struct {
	svc       *core.SSHKeyService
	tenantSvc *core.TenantService
}

// NewSSHKey creates a new SSHKey handler.
func NewSSHKey(svc *core.SSHKeyService, tenantSvc *core.TenantService) *SSHKey {
	return &SSHKey{svc: svc, tenantSvc: tenantSvc}
}

// ListByTenant godoc
//
//	@Summary		List SSH keys for a tenant
//	@Description	Returns a paginated list of SSH public keys associated with the specified tenant.
//	@Tags			SSH Keys
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.SSHKey}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/ssh-keys [get]
func (h *SSHKey) ListByTenant(w http.ResponseWriter, r *http.Request) {
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
//	@Summary		Create an SSH key
//	@Description	Adds an SSH public key for SFTP access to a tenant's files. The key fingerprint is computed server-side. Async — returns 202 and triggers a workflow to deploy the key to all nodes in the shard.
//	@Tags			SSH Keys
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			body body request.CreateSSHKey true "SSH key details"
//	@Success		202 {object} model.SSHKey
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/ssh-keys [post]
func (h *SSHKey) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateSSHKey
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
	key := &model.SSHKey{
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
//	@Summary		Get an SSH key
//	@Description	Returns a single SSH key by ID, including its public key and computed fingerprint.
//	@Tags			SSH Keys
//	@Security		ApiKeyAuth
//	@Param			id path string true "SSH key ID"
//	@Success		200 {object} model.SSHKey
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/ssh-keys/{id} [get]
func (h *SSHKey) Get(w http.ResponseWriter, r *http.Request) {
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
//	@Summary		Delete an SSH key
//	@Description	Removes an SSH key from all nodes in the tenant's shard. Async — returns 202 and triggers a Temporal workflow.
//	@Tags			SSH Keys
//	@Security		ApiKeyAuth
//	@Param			id path string true "SSH key ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/ssh-keys/{id} [delete]
func (h *SSHKey) Delete(w http.ResponseWriter, r *http.Request) {
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
//	@Summary		Retry a failed SSH key
//	@Description	Re-triggers the deployment workflow for an SSH key in failed state. Async — returns 202 and starts a new Temporal workflow.
//	@Tags			SSH Keys
//	@Security		ApiKeyAuth
//	@Param			id path string true "SSH key ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/ssh-keys/{id}/retry [post]
func (h *SSHKey) Retry(w http.ResponseWriter, r *http.Request) {
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
