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

type Backup struct {
	svc     *core.BackupService
	webroot *core.WebrootService
	db      *core.DatabaseService
}

func NewBackup(svc *core.BackupService, webroot *core.WebrootService, db *core.DatabaseService) *Backup {
	return &Backup{svc: svc, webroot: webroot, db: db}
}

// ListByTenant godoc
//
//	@Summary		List backups for a tenant
//	@Tags			Backups
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.Backup}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/backups [get]
func (h *Backup) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	backups, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(backups) > 0 {
		nextCursor = backups[len(backups)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, backups, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a backup
//	@Tags			Backups
//	@Security		ApiKeyAuth
//	@Param			tenantID path string true "Tenant ID"
//	@Param			body body request.CreateBackup true "Backup details"
//	@Success		202 {object} model.Backup
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/tenants/{tenantID}/backups [post]
func (h *Backup) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateBackup
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Resolve source name from the source resource.
	var sourceName string
	switch req.Type {
	case model.BackupTypeWeb:
		webroot, err := h.webroot.GetByID(r.Context(), req.SourceID)
		if err != nil {
			response.WriteError(w, http.StatusNotFound, "webroot not found: "+err.Error())
			return
		}
		sourceName = webroot.Name
	case model.BackupTypeDatabase:
		database, err := h.db.GetByID(r.Context(), req.SourceID)
		if err != nil {
			response.WriteError(w, http.StatusNotFound, "database not found: "+err.Error())
			return
		}
		sourceName = database.Name
	}

	now := time.Now()
	backup := &model.Backup{
		ID:         platform.NewID(),
		TenantID:   tenantID,
		Type:       req.Type,
		SourceID:   req.SourceID,
		SourceName: sourceName,
		Status:     model.StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.svc.Create(r.Context(), backup); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, backup)
}

// Get godoc
//
//	@Summary		Get a backup
//	@Tags			Backups
//	@Security		ApiKeyAuth
//	@Param			id path string true "Backup ID"
//	@Success		200 {object} model.Backup
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/backups/{id} [get]
func (h *Backup) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	backup, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, backup)
}

// Delete godoc
//
//	@Summary		Delete a backup
//	@Tags			Backups
//	@Security		ApiKeyAuth
//	@Param			id path string true "Backup ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/backups/{id} [delete]
func (h *Backup) Delete(w http.ResponseWriter, r *http.Request) {
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

// Restore godoc
//
//	@Summary		Restore a backup
//	@Tags			Backups
//	@Security		ApiKeyAuth
//	@Param			id path string true "Backup ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/backups/{id}/restore [post]
func (h *Backup) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Restore(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
