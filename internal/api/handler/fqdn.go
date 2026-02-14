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

type FQDN struct {
	svc      *core.FQDNService
	services *core.Services
}

func NewFQDN(services *core.Services) *FQDN {
	return &FQDN{svc: services.FQDN, services: services}
}

// ListByWebroot godoc
//
//	@Summary		List FQDNs for a webroot
//	@Tags			FQDNs
//	@Security		ApiKeyAuth
//	@Param			webrootID path string true "Webroot ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.FQDN}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/webroots/{webrootID}/fqdns [get]
func (h *FQDN) ListByWebroot(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	fqdns, hasMore, err := h.svc.ListByWebroot(r.Context(), webrootID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(fqdns) > 0 {
		nextCursor = fqdns[len(fqdns)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, fqdns, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an FQDN
//	@Tags			FQDNs
//	@Security		ApiKeyAuth
//	@Param			webrootID path string true "Webroot ID"
//	@Param			body body request.CreateFQDN true "FQDN details"
//	@Success		202 {object} model.FQDN
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/webroots/{webrootID}/fqdns [post]
func (h *FQDN) Create(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateFQDN
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	fqdn := &model.FQDN{
		ID:        platform.NewID(),
		FQDN:      req.FQDN,
		WebrootID: webrootID,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if req.SSLEnabled != nil {
		fqdn.SSLEnabled = *req.SSLEnabled
	}

	if err := h.svc.Create(r.Context(), fqdn); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Nested email account creation
	if err := createNestedEmailAccounts(r.Context(), h.services, fqdn.ID, req.EmailAccounts); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, fqdn)
}

// Get godoc
//
//	@Summary		Get an FQDN
//	@Tags			FQDNs
//	@Security		ApiKeyAuth
//	@Param			id path string true "FQDN ID"
//	@Success		200 {object} model.FQDN
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/fqdns/{id} [get]
func (h *FQDN) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fqdn, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, fqdn)
}

// Delete godoc
//
//	@Summary		Delete an FQDN
//	@Tags			FQDNs
//	@Security		ApiKeyAuth
//	@Param			id path string true "FQDN ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/fqdns/{id} [delete]
func (h *FQDN) Delete(w http.ResponseWriter, r *http.Request) {
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

// Retry godoc
//
//	@Summary		Retry a failed FQDN
//	@Tags			FQDNs
//	@Security		ApiKeyAuth
//	@Param			id path string true "FQDN ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/fqdns/{id}/retry [post]
func (h *FQDN) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
