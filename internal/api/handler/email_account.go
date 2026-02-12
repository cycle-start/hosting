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

type EmailAccount struct {
	svc *core.EmailAccountService
}

func NewEmailAccount(svc *core.EmailAccountService) *EmailAccount {
	return &EmailAccount{svc: svc}
}

func (h *EmailAccount) ListByFQDN(w http.ResponseWriter, r *http.Request) {
	fqdnID, err := request.RequireID(chi.URLParam(r, "fqdnID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	accounts, hasMore, err := h.svc.ListByFQDN(r.Context(), fqdnID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(accounts) > 0 {
		nextCursor = accounts[len(accounts)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, accounts, nextCursor, hasMore)
}

func (h *EmailAccount) Create(w http.ResponseWriter, r *http.Request) {
	fqdnID, err := request.RequireID(chi.URLParam(r, "fqdnID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateEmailAccount
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	account := &model.EmailAccount{
		ID:          platform.NewID(),
		FQDNID:      fqdnID,
		Address:     req.Address,
		DisplayName: req.DisplayName,
		QuotaBytes:  req.QuotaBytes,
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.svc.Create(r.Context(), account); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, account)
}

func (h *EmailAccount) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, account)
}

func (h *EmailAccount) Delete(w http.ResponseWriter, r *http.Request) {
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
