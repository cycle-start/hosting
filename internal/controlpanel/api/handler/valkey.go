package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type ValkeyHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewValkeyHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *ValkeyHandler {
	return &ValkeyHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

// ListByCustomer fetches Valkey instances across all tenants the customer has subscriptions with the "valkey" module.
//
//	@Summary      List Valkey instances by customer
//	@Description  Fetches Valkey instances across all tenants the customer has subscriptions with the "valkey" module
//	@Tags         Valkey
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.ValkeyInstance
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/valkey [get]
func (h *ValkeyHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	listByCustomerWithModule[hosting.ValkeyInstance](w, r, h.customerSvc, h.subscriptionSvc, "valkey", func(tid string) ([]hosting.ValkeyInstance, error) {
		return h.hostingClient.ListValkeyInstancesByTenant(r.Context(), tid)
	})
}

// Get returns a single Valkey instance by ID, with authorization check.
//
//	@Summary      Get a Valkey instance
//	@Description  Returns a single Valkey instance by ID, with authorization check
//	@Tags         Valkey
//	@Produce      json
//	@Param        id   path      string  true  "Valkey Instance ID"
//	@Success      200  {object}  hosting.ValkeyInstance
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /valkey/{id} [get]
func (h *ValkeyHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	instance, err := h.hostingClient.GetValkeyInstance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "valkey instance not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, instance.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, instance)
}

// Delete removes a Valkey instance by ID.
//
//	@Summary      Delete a Valkey instance
//	@Description  Removes a Valkey instance by ID
//	@Tags         Valkey
//	@Produce      json
//	@Param        id   path      string  true  "Valkey Instance ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /valkey/{id} [delete]
func (h *ValkeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	instance, err := h.hostingClient.GetValkeyInstance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "valkey instance not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, instance.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteValkeyInstance(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete valkey instance")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListUsers returns all users for a Valkey instance.
//
//	@Summary      List Valkey users
//	@Description  Returns all users for a Valkey instance
//	@Tags         Valkey
//	@Produce      json
//	@Param        id   path      string  true  "Valkey Instance ID"
//	@Success      200  {object}  map[string][]hosting.ValkeyUser
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /valkey/{id}/users [get]
func (h *ValkeyHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	instance, err := h.hostingClient.GetValkeyInstance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "valkey instance not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, instance.TenantID) {
		return
	}

	users, err := h.hostingClient.ListValkeyUsers(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch valkey users")
		return
	}
	if users == nil {
		users = []hosting.ValkeyUser{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": users})
}

// CreateUser creates a user for a Valkey instance.
//
//	@Summary      Create a Valkey user
//	@Description  Creates a user for a Valkey instance
//	@Tags         Valkey
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Valkey Instance ID"
//	@Param        body  body      object  true  "User creation payload"
//	@Success      201   {object}  hosting.ValkeyUser
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /valkey/{id}/users [post]
func (h *ValkeyHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	instance, err := h.hostingClient.GetValkeyInstance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "valkey instance not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, instance.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.hostingClient.CreateValkeyUser(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create valkey user")
		return
	}

	response.WriteJSON(w, http.StatusCreated, user)
}

// UpdateUser updates a Valkey user.
//
//	@Summary      Update a Valkey user
//	@Description  Updates a Valkey user
//	@Tags         Valkey
//	@Accept       json
//	@Produce      json
//	@Param        id      path      string  true  "Valkey Instance ID"
//	@Param        userId  path      string  true  "User ID"
//	@Param        body    body      object  true  "User update payload"
//	@Success      200     {object}  hosting.ValkeyUser
//	@Failure      400     {object}  response.ErrorResponse
//	@Failure      403     {object}  response.ErrorResponse
//	@Failure      404     {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /valkey/{id}/users/{userId} [put]
func (h *ValkeyHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	instance, err := h.hostingClient.GetValkeyInstance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "valkey instance not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, instance.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := chi.URLParam(r, "userId")
	user, err := h.hostingClient.UpdateValkeyUser(r.Context(), userID, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update valkey user")
		return
	}

	response.WriteJSON(w, http.StatusOK, user)
}

// DeleteUser deletes a Valkey user.
//
//	@Summary      Delete a Valkey user
//	@Description  Deletes a Valkey user
//	@Tags         Valkey
//	@Produce      json
//	@Param        id      path      string  true  "Valkey Instance ID"
//	@Param        userId  path      string  true  "User ID"
//	@Success      204     "No Content"
//	@Failure      400     {object}  response.ErrorResponse
//	@Failure      403     {object}  response.ErrorResponse
//	@Failure      404     {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /valkey/{id}/users/{userId} [delete]
func (h *ValkeyHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	instance, err := h.hostingClient.GetValkeyInstance(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "valkey instance not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, instance.TenantID) {
		return
	}

	userID := chi.URLParam(r, "userId")
	if err := h.hostingClient.DeleteValkeyUser(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete valkey user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
