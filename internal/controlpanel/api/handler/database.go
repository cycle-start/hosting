package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type DatabaseHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewDatabaseHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *DatabaseHandler {
	return &DatabaseHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

// ListByCustomer fetches databases across all tenants the customer has subscriptions with the "databases" module.
//
//	@Summary      List databases by customer
//	@Description  Fetches databases across all tenants the customer has subscriptions with the "databases" module
//	@Tags         Databases
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.Database
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/databases [get]
func (h *DatabaseHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	listByCustomerWithModule[hosting.Database](w, r, h.customerSvc, h.subscriptionSvc, "databases", func(tid string) ([]hosting.Database, error) {
		return h.hostingClient.ListDatabasesByTenant(r.Context(), tid)
	})
}

// Get returns a single database by ID, with authorization check.
//
//	@Summary      Get a database
//	@Description  Returns a single database by ID, with authorization check
//	@Tags         Databases
//	@Produce      json
//	@Param        id   path      string  true  "Database ID"
//	@Success      200  {object}  hosting.Database
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /databases/{id} [get]
func (h *DatabaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := h.hostingClient.GetDatabase(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, db.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, db)
}

// Delete removes a database by ID.
//
//	@Summary      Delete a database
//	@Description  Removes a database by ID
//	@Tags         Databases
//	@Produce      json
//	@Param        id   path      string  true  "Database ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /databases/{id} [delete]
func (h *DatabaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := h.hostingClient.GetDatabase(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, db.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteDatabase(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete database")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListUsers returns all users for a database.
//
//	@Summary      List database users
//	@Description  Returns all users for a database
//	@Tags         Databases
//	@Produce      json
//	@Param        id   path      string  true  "Database ID"
//	@Success      200  {object}  map[string][]hosting.DatabaseUser
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /databases/{id}/users [get]
func (h *DatabaseHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := h.hostingClient.GetDatabase(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, db.TenantID) {
		return
	}

	users, err := h.hostingClient.ListDatabaseUsers(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch database users")
		return
	}
	if users == nil {
		users = []hosting.DatabaseUser{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": users})
}

// CreateUser creates a user for a database.
//
//	@Summary      Create a database user
//	@Description  Creates a user for a database
//	@Tags         Databases
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Database ID"
//	@Param        body  body      object  true  "User creation payload"
//	@Success      201   {object}  hosting.DatabaseUser
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /databases/{id}/users [post]
func (h *DatabaseHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := h.hostingClient.GetDatabase(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, db.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.hostingClient.CreateDatabaseUser(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create database user")
		return
	}

	response.WriteJSON(w, http.StatusCreated, user)
}

// UpdateUser updates a database user.
//
//	@Summary      Update a database user
//	@Description  Updates a database user
//	@Tags         Databases
//	@Accept       json
//	@Produce      json
//	@Param        id      path      string  true  "Database ID"
//	@Param        userId  path      string  true  "User ID"
//	@Param        body    body      object  true  "User update payload"
//	@Success      200     {object}  hosting.DatabaseUser
//	@Failure      400     {object}  response.ErrorResponse
//	@Failure      403     {object}  response.ErrorResponse
//	@Failure      404     {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /databases/{id}/users/{userId} [put]
func (h *DatabaseHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := h.hostingClient.GetDatabase(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, db.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	userID := chi.URLParam(r, "userId")
	user, err := h.hostingClient.UpdateDatabaseUser(r.Context(), userID, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update database user")
		return
	}

	response.WriteJSON(w, http.StatusOK, user)
}

// DeleteUser deletes a database user.
//
//	@Summary      Delete a database user
//	@Description  Deletes a database user
//	@Tags         Databases
//	@Produce      json
//	@Param        id      path      string  true  "Database ID"
//	@Param        userId  path      string  true  "User ID"
//	@Success      204     "No Content"
//	@Failure      400     {object}  response.ErrorResponse
//	@Failure      403     {object}  response.ErrorResponse
//	@Failure      404     {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /databases/{id}/users/{userId} [delete]
func (h *DatabaseHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := h.hostingClient.GetDatabase(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, db.TenantID) {
		return
	}

	userID := chi.URLParam(r, "userId")
	if err := h.hostingClient.DeleteDatabaseUser(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete database user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateLoginSession creates a short-lived login session for opening DB Admin.
//
//	@Summary      Create a DB Admin login session
//	@Description  Creates a short-lived login session that can be used to authenticate with the DB Admin UI
//	@Tags         Databases
//	@Produce      json
//	@Param        id   path      string  true  "Database ID"
//	@Success      200  {object}  hosting.LoginSession
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /databases/{id}/login-session [post]
func (h *DatabaseHandler) CreateLoginSession(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	db, err := h.hostingClient.GetDatabase(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "database not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, db.TenantID) {
		return
	}

	session, err := h.hostingClient.CreateLoginSession(r.Context(), db.TenantID, id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create login session")
		return
	}

	response.WriteJSON(w, http.StatusOK, session)
}
