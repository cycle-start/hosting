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

func (h *EmailForward) ListByAccount(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	forwards, err := h.svc.ListByAccountID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, forwards)
}

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
