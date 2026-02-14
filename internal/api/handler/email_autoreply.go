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

type EmailAutoReply struct {
	svc *core.EmailAutoReplyService
}

func NewEmailAutoReply(svc *core.EmailAutoReplyService) *EmailAutoReply {
	return &EmailAutoReply{svc: svc}
}

// Get godoc
//
//	@Summary		Get email auto-reply settings
//	@Description	Returns the current autoreply configuration for the specified email account. Returns 404 if no autoreply is configured.
//	@Tags			Email Auto-Reply
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Success		200 {object} model.EmailAutoReply
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/autoreply [get]
func (h *EmailAutoReply) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	ar, err := h.svc.GetByAccountID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, ar)
}

// Put godoc
//
//	@Summary		Update email auto-reply settings
//	@Description	Asynchronously creates or replaces the autoreply (out-of-office) message for an email account. Supports optional start/end dates for time-limited autoreplies and an enabled flag. Triggers a Temporal workflow to configure in Stalwart. Returns 202 Accepted.
//	@Tags			Email Auto-Reply
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Param			body body request.UpdateEmailAutoReply true "Auto-reply settings"
//	@Success		202 {object} model.EmailAutoReply
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/autoreply [put]
func (h *EmailAutoReply) Put(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateEmailAutoReply
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	ar := &model.EmailAutoReply{
		ID:             platform.NewID(),
		EmailAccountID: id,
		Subject:        req.Subject,
		Body:           req.Body,
		StartDate:      req.StartDate,
		EndDate:        req.EndDate,
		Enabled:        req.Enabled,
		Status:         model.StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.svc.Upsert(r.Context(), ar); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, ar)
}

// Delete godoc
//
//	@Summary		Delete email auto-reply settings
//	@Description	Asynchronously removes the autoreply configuration from the email account. Triggers a Temporal workflow to remove from Stalwart. Returns 202 Accepted.
//	@Tags			Email Auto-Reply
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/autoreply [delete]
func (h *EmailAutoReply) Delete(w http.ResponseWriter, r *http.Request) {
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
//	@Summary		Retry a failed email auto-reply
//	@Description	Re-triggers the provisioning workflow for an email autoreply that is in a failed state. Returns 202 Accepted.
//	@Tags			Email Auto-Replies
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email auto-reply ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-autoreplies/{id}/retry [post]
func (h *EmailAutoReply) Retry(w http.ResponseWriter, r *http.Request) {
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
