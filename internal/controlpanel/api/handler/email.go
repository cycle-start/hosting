package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type EmailHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewEmailHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *EmailHandler {
	return &EmailHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

// ListByCustomer fetches email accounts across all tenants the customer has subscriptions with the "email" module.
//
//	@Summary      List email accounts by customer
//	@Description  Fetches email accounts across all tenants the customer has subscriptions with the "email" module
//	@Tags         Email
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.EmailAccount
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/email [get]
func (h *EmailHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	listByCustomerWithModule[hosting.EmailAccount](w, r, h.customerSvc, h.subscriptionSvc, "email", func(tid string) ([]hosting.EmailAccount, error) {
		return h.hostingClient.ListEmailAccountsByTenant(r.Context(), tid)
	})
}

// Get returns a single email account by ID, with authorization check.
//
//	@Summary      Get an email account
//	@Description  Returns a single email account by ID, with authorization check
//	@Tags         Email
//	@Produce      json
//	@Param        id   path      string  true  "Email Account ID"
//	@Success      200  {object}  hosting.EmailAccount
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id} [get]
func (h *EmailHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, account)
}

// Delete removes an email account by ID.
//
//	@Summary      Delete an email account
//	@Description  Removes an email account by ID
//	@Tags         Email
//	@Produce      json
//	@Param        id   path      string  true  "Email Account ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id} [delete]
func (h *EmailHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	if err := h.hostingClient.DeleteEmailAccount(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete email account")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAliases returns all aliases for an email account.
//
//	@Summary      List email aliases
//	@Description  Returns all aliases for an email account
//	@Tags         Email
//	@Produce      json
//	@Param        id   path      string  true  "Email Account ID"
//	@Success      200  {object}  map[string][]hosting.EmailAlias
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/aliases [get]
func (h *EmailHandler) ListAliases(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	aliases, err := h.hostingClient.ListEmailAliases(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch email aliases")
		return
	}
	if aliases == nil {
		aliases = []hosting.EmailAlias{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": aliases})
}

// CreateAlias creates an alias for an email account.
//
//	@Summary      Create an email alias
//	@Description  Creates an alias for an email account
//	@Tags         Email
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Email Account ID"
//	@Param        body  body      object  true  "Alias creation payload"
//	@Success      201   {object}  hosting.EmailAlias
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/aliases [post]
func (h *EmailHandler) CreateAlias(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	alias, err := h.hostingClient.CreateEmailAlias(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create email alias")
		return
	}

	response.WriteJSON(w, http.StatusCreated, alias)
}

// DeleteAlias deletes an alias for an email account.
//
//	@Summary      Delete an email alias
//	@Description  Deletes an alias for an email account
//	@Tags         Email
//	@Produce      json
//	@Param        id       path      string  true  "Email Account ID"
//	@Param        aliasId  path      string  true  "Alias ID"
//	@Success      204      "No Content"
//	@Failure      400      {object}  response.ErrorResponse
//	@Failure      403      {object}  response.ErrorResponse
//	@Failure      404      {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/aliases/{aliasId} [delete]
func (h *EmailHandler) DeleteAlias(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	aliasID := chi.URLParam(r, "aliasId")
	if err := h.hostingClient.DeleteEmailAlias(r.Context(), aliasID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete email alias")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListForwards returns all forwards for an email account.
//
//	@Summary      List email forwards
//	@Description  Returns all forwards for an email account
//	@Tags         Email
//	@Produce      json
//	@Param        id   path      string  true  "Email Account ID"
//	@Success      200  {object}  map[string][]hosting.EmailForward
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/forwards [get]
func (h *EmailHandler) ListForwards(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	forwards, err := h.hostingClient.ListEmailForwards(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch email forwards")
		return
	}
	if forwards == nil {
		forwards = []hosting.EmailForward{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": forwards})
}

// CreateForward creates a forward for an email account.
//
//	@Summary      Create an email forward
//	@Description  Creates a forward for an email account
//	@Tags         Email
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Email Account ID"
//	@Param        body  body      object  true  "Forward creation payload"
//	@Success      201   {object}  hosting.EmailForward
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/forwards [post]
func (h *EmailHandler) CreateForward(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	forward, err := h.hostingClient.CreateEmailForward(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create email forward")
		return
	}

	response.WriteJSON(w, http.StatusCreated, forward)
}

// DeleteForward deletes a forward for an email account.
//
//	@Summary      Delete an email forward
//	@Description  Deletes a forward for an email account
//	@Tags         Email
//	@Produce      json
//	@Param        id         path      string  true  "Email Account ID"
//	@Param        forwardId  path      string  true  "Forward ID"
//	@Success      204        "No Content"
//	@Failure      400        {object}  response.ErrorResponse
//	@Failure      403        {object}  response.ErrorResponse
//	@Failure      404        {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/forwards/{forwardId} [delete]
func (h *EmailHandler) DeleteForward(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	forwardID := chi.URLParam(r, "forwardId")
	if err := h.hostingClient.DeleteEmailForward(r.Context(), forwardID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete email forward")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetAutoreply returns the autoreply for an email account.
//
//	@Summary      Get email autoreply
//	@Description  Returns the autoreply for an email account
//	@Tags         Email
//	@Produce      json
//	@Param        id   path      string  true  "Email Account ID"
//	@Success      200  {object}  hosting.EmailAutoreply
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/autoreply [get]
func (h *EmailHandler) GetAutoreply(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	autoreply, err := h.hostingClient.GetEmailAutoreply(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "autoreply not found")
		return
	}

	response.WriteJSON(w, http.StatusOK, autoreply)
}

// SetAutoreply sets the autoreply for an email account.
//
//	@Summary      Set email autoreply
//	@Description  Sets the autoreply for an email account
//	@Tags         Email
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Email Account ID"
//	@Param        body  body      object  true  "Autoreply payload"
//	@Success      200   {object}  hosting.EmailAutoreply
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/autoreply [put]
func (h *EmailHandler) SetAutoreply(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	autoreply, err := h.hostingClient.SetEmailAutoreply(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to set autoreply")
		return
	}

	response.WriteJSON(w, http.StatusOK, autoreply)
}

// DeleteAutoreply deletes the autoreply for an email account.
//
//	@Summary      Delete email autoreply
//	@Description  Deletes the autoreply for an email account
//	@Tags         Email
//	@Produce      json
//	@Param        id   path      string  true  "Email Account ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /email/{id}/autoreply [delete]
func (h *EmailHandler) DeleteAutoreply(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	account, err := h.hostingClient.GetEmailAccount(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "email account not found")
		return
	}

	if !authorizeResourceBySubscription(w, r, h.customerSvc, account.SubscriptionID) {
		return
	}

	if err := h.hostingClient.DeleteEmailAutoreply(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete autoreply")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
