package handler

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type ValkeyUser struct {
	svc         *core.ValkeyUserService
	instanceSvc *core.ValkeyInstanceService
}

func NewValkeyUser(svc *core.ValkeyUserService, instanceSvc *core.ValkeyInstanceService) *ValkeyUser {
	return &ValkeyUser{svc: svc, instanceSvc: instanceSvc}
}

// ListByInstance godoc
//
//	@Summary		List Valkey users
//	@Description	Returns a paginated list of Valkey ACL users for an instance. Passwords are redacted in list responses.
//	@Tags			Valkey Users
//	@Security		ApiKeyAuth
//	@Param			instanceID	path		string	true	"Valkey instance ID"
//	@Param			limit		query		int		false	"Page size"	default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.ValkeyUser}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/valkey-instances/{instanceID}/users [get]
func (h *ValkeyUser) ListByInstance(w http.ResponseWriter, r *http.Request) {
	instanceID, err := request.RequireID(chi.URLParam(r, "instanceID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	users, hasMore, err := h.svc.ListByInstance(r.Context(), instanceID, pg.Limit, pg.Cursor)
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
//	@Summary		Create a Valkey user
//	@Description	Asynchronously creates a Valkey ACL user with username, password, privileges, and key pattern. Key pattern defaults to "~*" (all keys). Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			Valkey Users
//	@Security		ApiKeyAuth
//	@Param			instanceID	path		string						true	"Valkey instance ID"
//	@Param			body		body		request.CreateValkeyUser	true	"Valkey user details"
//	@Success		202			{object}	model.ValkeyUser
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/valkey-instances/{instanceID}/users [post]
func (h *ValkeyUser) Create(w http.ResponseWriter, r *http.Request) {
	instanceID, err := request.RequireID(chi.URLParam(r, "instanceID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateValkeyUser
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate username starts with parent instance name.
	instance, err := h.instanceSvc.GetByID(r.Context(), instanceID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !strings.HasPrefix(req.Username, instance.Name) {
		response.WriteError(w, http.StatusBadRequest, fmt.Sprintf("username %q must start with instance name %q", req.Username, instance.Name))
		return
	}

	keyPattern := req.KeyPattern
	if keyPattern == "" {
		keyPattern = "~*"
	}

	now := time.Now()
	user := &model.ValkeyUser{
		ID:               platform.NewID(),
		ValkeyInstanceID: instanceID,
		Username:         req.Username,
		Password:         req.Password,
		Privileges:       req.Privileges,
		KeyPattern:       keyPattern,
		Status:           model.StatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
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
//	@Summary		Get a Valkey user
//	@Description	Returns the details of a single Valkey user by ID. The password is redacted.
//	@Tags			Valkey Users
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Valkey user ID"
//	@Success		200	{object}	model.ValkeyUser
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/valkey-users/{id} [get]
func (h *ValkeyUser) Get(w http.ResponseWriter, r *http.Request) {
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
//	@Summary		Update a Valkey user
//	@Description	Asynchronously updates a Valkey user's password, privileges, or key pattern. Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			Valkey Users
//	@Security		ApiKeyAuth
//	@Param			id		path		string						true	"Valkey user ID"
//	@Param			body	body		request.UpdateValkeyUser	true	"Valkey user updates"
//	@Success		202		{object}	model.ValkeyUser
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/valkey-users/{id} [put]
func (h *ValkeyUser) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateValkeyUser
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
	if req.KeyPattern != "" {
		user.KeyPattern = req.KeyPattern
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
//	@Summary		Delete a Valkey user
//	@Description	Asynchronously removes a Valkey ACL user from the instance. Triggers a Temporal workflow and returns 202 immediately.
//	@Tags			Valkey Users
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Valkey user ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/valkey-users/{id} [delete]
func (h *ValkeyUser) Delete(w http.ResponseWriter, r *http.Request) {
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
//	@Summary		Retry a failed Valkey user
//	@Description	Re-triggers the provisioning workflow for a Valkey user that previously failed. Returns 202 immediately.
//	@Tags			Valkey Users
//	@Security		ApiKeyAuth
//	@Param			id path string true "Valkey user ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/valkey-users/{id}/retry [post]
func (h *ValkeyUser) Retry(w http.ResponseWriter, r *http.Request) {
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
