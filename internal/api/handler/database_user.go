package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type DatabaseUser struct {
	svc   *core.DatabaseUserService
	dbSvc *core.DatabaseService
}

func NewDatabaseUser(svc *core.DatabaseUserService, dbSvc *core.DatabaseService) *DatabaseUser {
	return &DatabaseUser{svc: svc, dbSvc: dbSvc}
}

// ListByDatabase godoc
//
//	@Summary		List database users
//	@Description	Returns a paginated list of users for the specified database. Passwords are not included in the response.
//	@Tags			Database Users
//	@Security		ApiKeyAuth
//	@Param			databaseID	path		string	true	"Database ID"
//	@Param			limit		query		int		false	"Page size"	default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.DatabaseUser}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/databases/{databaseID}/users [get]
func (h *DatabaseUser) ListByDatabase(w http.ResponseWriter, r *http.Request) {
	databaseID, err := request.RequireID(chi.URLParam(r, "databaseID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	users, hasMore, err := h.svc.ListByDatabase(r.Context(), databaseID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	for i := range users {
		users[i].Password = ""
	}
	var nextCursor string
	if hasMore && len(users) > 0 {
		nextCursor = users[len(users)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, users, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a database user
//	@Description	Creates a MySQL user with the given username, password, and privileges on the parent database. Returns 202 and triggers a Temporal workflow to provision the user on the MySQL node. The password is not returned in subsequent GET requests.
//	@Tags			Database Users
//	@Security		ApiKeyAuth
//	@Param			databaseID	path		string						true	"Database ID"
//	@Param			body		body		request.CreateDatabaseUser	true	"Database user details"
//	@Success		202			{object}	model.DatabaseUser
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/databases/{databaseID}/users [post]
func (h *DatabaseUser) Create(w http.ResponseWriter, r *http.Request) {
	databaseID, err := request.RequireID(chi.URLParam(r, "databaseID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateDatabaseUser
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate username starts with parent database name.
	db, err := h.dbSvc.GetByID(r.Context(), databaseID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !strings.HasPrefix(req.Username, db.Name) {
		response.WriteError(w, http.StatusBadRequest, fmt.Sprintf("username %q must start with database name %q", req.Username, db.Name))
		return
	}

	now := time.Now()
	user := &model.DatabaseUser{
		ID:         platform.NewID(),
		DatabaseID: databaseID,
		Username:   req.Username,
		Password:   req.Password,
		Privileges: req.Privileges,
		Status:     model.StatusPending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.svc.Create(r.Context(), user); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusAccepted, user)
}

// Get godoc
//
//	@Summary		Get a database user
//	@Description	Returns the details of a single database user. The password is never included in the response after initial creation.
//	@Tags			Database Users
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Database user ID"
//	@Success		200	{object}	model.DatabaseUser
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/database-users/{id} [get]
func (h *DatabaseUser) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusOK, user)
}

// Update godoc
//
//	@Summary		Update a database user
//	@Description	Updates the password and/or privileges of a database user. Returns 202 and triggers a Temporal workflow to apply the changes on the MySQL node. Only provided fields are updated.
//	@Tags			Database Users
//	@Security		ApiKeyAuth
//	@Param			id		path		string						true	"Database user ID"
//	@Param			body	body		request.UpdateDatabaseUser	true	"Database user updates"
//	@Success		202		{object}	model.DatabaseUser
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/database-users/{id} [put]
func (h *DatabaseUser) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateDatabaseUser
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	user, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Password != "" {
		user.Password = req.Password
	}
	if req.Privileges != nil {
		user.Privileges = req.Privileges
	}

	if err := h.svc.Update(r.Context(), user); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	user.Password = ""
	response.WriteJSON(w, http.StatusAccepted, user)
}

// Delete godoc
//
//	@Summary		Delete a database user
//	@Description	Drops the MySQL user from the database node. Returns 202 and triggers a Temporal workflow to remove the user.
//	@Tags			Database Users
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Database user ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/database-users/{id} [delete]
func (h *DatabaseUser) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Retry godoc
//
//	@Summary		Retry a failed database user
//	@Description	Re-triggers the provisioning workflow for a database user that is in a failed state. Returns 202 and starts a new Temporal workflow.
//	@Tags			Database Users
//	@Security		ApiKeyAuth
//	@Param			id path string true "Database user ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/database-users/{id}/retry [post]
func (h *DatabaseUser) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}
