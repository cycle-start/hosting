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

type EmailForward struct {
	svc *core.EmailForwardService
}

func NewEmailForward(svc *core.EmailForwardService) *EmailForward {
	return &EmailForward{svc: svc}
}

// ListByAccount godoc
//
//	@Summary		List email forwards for an account
//	@Tags			Email Forwards
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.EmailForward}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/forwards [get]
func (h *EmailForward) ListByAccount(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	forwards, hasMore, err := h.svc.ListByAccountID(r.Context(), id, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(forwards) > 0 {
		nextCursor = forwards[len(forwards)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, forwards, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create an email forward
//	@Tags			Email Forwards
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Param			body body request.CreateEmailForward true "Email forward details"
//	@Success		202 {object} model.EmailForward
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/forwards [post]
func (h *EmailForward) Create(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateEmailForward
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	keepCopy := true
	if req.KeepCopy != nil {
		keepCopy = *req.KeepCopy
	}

	now := time.Now()
	fwd := &model.EmailForward{
		ID:             platform.NewID(),
		EmailAccountID: id,
		Destination:    req.Destination,
		KeepCopy:       keepCopy,
		Status:         model.StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.svc.Create(r.Context(), fwd); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, fwd)
}

// Get godoc
//
//	@Summary		Get an email forward
//	@Tags			Email Forwards
//	@Security		ApiKeyAuth
//	@Param			forwardID path string true "Email forward ID"
//	@Success		200 {object} model.EmailForward
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/email-forwards/{forwardID} [get]
func (h *EmailForward) Get(w http.ResponseWriter, r *http.Request) {
	forwardID, err := request.RequireID(chi.URLParam(r, "forwardID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fwd, err := h.svc.GetByID(r.Context(), forwardID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, fwd)
}

// Delete godoc
//
//	@Summary		Delete an email forward
//	@Tags			Email Forwards
//	@Security		ApiKeyAuth
//	@Param			forwardID path string true "Email forward ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-forwards/{forwardID} [delete]
func (h *EmailForward) Delete(w http.ResponseWriter, r *http.Request) {
	forwardID, err := request.RequireID(chi.URLParam(r, "forwardID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), forwardID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Retry godoc
//
//	@Summary		Retry a failed email forward
//	@Tags			Email Forwards
//	@Security		ApiKeyAuth
//	@Param			forwardID path string true "Email forward ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-forwards/{forwardID}/retry [post]
func (h *EmailForward) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "forwardID"))
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
