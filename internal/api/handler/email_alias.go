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

type EmailAlias struct {
	svc *core.EmailAliasService
}

func NewEmailAlias(svc *core.EmailAliasService) *EmailAlias {
	return &EmailAlias{svc: svc}
}

// ListByAccount godoc
//
//	@Summary		List email aliases for an account
//	@Tags			Email Aliases
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.EmailAlias}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/aliases [get]
func (h *EmailAlias) ListByAccount(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	aliases, hasMore, err := h.svc.ListByAccountID(r.Context(), id, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(aliases) > 0 {
		nextCursor = aliases[len(aliases)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, aliases, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an email alias
//	@Tags			Email Aliases
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Param			body body request.CreateEmailAlias true "Email alias details"
//	@Success		202 {object} model.EmailAlias
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/aliases [post]
func (h *EmailAlias) Create(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateEmailAlias
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	alias := &model.EmailAlias{
		ID:             platform.NewID(),
		EmailAccountID: id,
		Address:        req.Address,
		Status:         model.StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.svc.Create(r.Context(), alias); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, alias)
}

// Get godoc
//
//	@Summary		Get an email alias
//	@Tags			Email Aliases
//	@Security		ApiKeyAuth
//	@Param			aliasID path string true "Email alias ID"
//	@Success		200 {object} model.EmailAlias
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/email-aliases/{aliasID} [get]
func (h *EmailAlias) Get(w http.ResponseWriter, r *http.Request) {
	aliasID, err := request.RequireID(chi.URLParam(r, "aliasID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	alias, err := h.svc.GetByID(r.Context(), aliasID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, alias)
}

// Delete godoc
//
//	@Summary		Delete an email alias
//	@Tags			Email Aliases
//	@Security		ApiKeyAuth
//	@Param			aliasID path string true "Email alias ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-aliases/{aliasID} [delete]
func (h *EmailAlias) Delete(w http.ResponseWriter, r *http.Request) {
	aliasID, err := request.RequireID(chi.URLParam(r, "aliasID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), aliasID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
