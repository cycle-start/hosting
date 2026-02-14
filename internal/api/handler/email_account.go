package handler

import (
	"fmt"
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
	svc      *core.EmailAccountService
	services *core.Services
}

func NewEmailAccount(services *core.Services) *EmailAccount {
	return &EmailAccount{svc: services.EmailAccount, services: services}
}

// ListByFQDN godoc
//
//	@Summary		List email accounts for an FQDN
//	@Tags			Email Accounts
//	@Security		ApiKeyAuth
//	@Param			fqdnID path string true "FQDN ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.EmailAccount}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/fqdns/{fqdnID}/email-accounts [get]
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

// Create godoc
//
//	@Summary		Create an email account
//	@Tags			Email Accounts
//	@Security		ApiKeyAuth
//	@Param			fqdnID path string true "FQDN ID"
//	@Param			body body request.CreateEmailAccount true "Email account details"
//	@Success		202 {object} model.EmailAccount
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/fqdns/{fqdnID}/email-accounts [post]
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

	// Nested alias creation
	for _, al := range req.Aliases {
		now2 := time.Now()
		alias := &model.EmailAlias{
			ID:             platform.NewID(),
			EmailAccountID: account.ID,
			Address:        al.Address,
			Status:         model.StatusPending,
			CreatedAt:      now2,
			UpdatedAt:      now2,
		}
		if err := h.services.EmailAlias.Create(r.Context(), alias); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create email alias %s: %s", al.Address, err.Error()))
			return
		}
	}

	// Nested forward creation
	for _, fw := range req.Forwards {
		keepCopy := true
		if fw.KeepCopy != nil {
			keepCopy = *fw.KeepCopy
		}
		now2 := time.Now()
		fwd := &model.EmailForward{
			ID:             platform.NewID(),
			EmailAccountID: account.ID,
			Destination:    fw.Destination,
			KeepCopy:       keepCopy,
			Status:         model.StatusPending,
			CreatedAt:      now2,
			UpdatedAt:      now2,
		}
		if err := h.services.EmailForward.Create(r.Context(), fwd); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create email forward %s: %s", fw.Destination, err.Error()))
			return
		}
	}

	// Nested auto-reply creation
	if req.AutoReply != nil {
		now2 := time.Now()
		autoReply := &model.EmailAutoReply{
			ID:             platform.NewID(),
			EmailAccountID: account.ID,
			Subject:        req.AutoReply.Subject,
			Body:           req.AutoReply.Body,
			StartDate:      req.AutoReply.StartDate,
			EndDate:        req.AutoReply.EndDate,
			Enabled:        req.AutoReply.Enabled,
			Status:         model.StatusPending,
			CreatedAt:      now2,
			UpdatedAt:      now2,
		}
		if err := h.services.EmailAutoReply.Upsert(r.Context(), autoReply); err != nil {
			response.WriteError(w, http.StatusInternalServerError, fmt.Sprintf("create email autoreply for %s: %s", req.Address, err.Error()))
			return
		}
	}

	response.WriteJSON(w, http.StatusAccepted, account)
}

// Get godoc
//
//	@Summary		Get an email account
//	@Tags			Email Accounts
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Success		200 {object} model.EmailAccount
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/email-accounts/{id} [get]
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

// Delete godoc
//
//	@Summary		Delete an email account
//	@Tags			Email Accounts
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id} [delete]
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

// Retry godoc
//
//	@Summary		Retry a failed email account
//	@Tags			Email Accounts
//	@Security		ApiKeyAuth
//	@Param			id path string true "Email account ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/email-accounts/{id}/retry [post]
func (h *EmailAccount) Retry(w http.ResponseWriter, r *http.Request) {
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
