package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Database struct {
	svc *core.DatabaseService
}

func NewDatabase(svc *core.DatabaseService) *Database {
	return &Database{svc: svc}
}

// ListByTenant godoc
//
//	@Summary		List databases for a tenant
//	@Tags			Databases
//	@Security		ApiKeyAuth
//	@Param			tenantID	path		string	true	"Tenant ID"
//	@Param			limit		query		int		false	"Page size"						default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Param			search		query		string	false	"Search term"
//	@Param			status		query		string	false	"Filter by status"
//	@Param			sort		query		string	false	"Sort field"					default(created_at)
//	@Param			order		query		string	false	"Sort order (asc or desc)"		default(asc)
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.Database}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/tenants/{tenantID}/databases [get]
func (h *Database) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	params := request.ParseListParams(r, "created_at")

	databases, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, params)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(databases) > 0 {
		nextCursor = databases[len(databases)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, databases, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a database
//	@Tags			Databases
//	@Security		ApiKeyAuth
//	@Param			tenantID	path		string					true	"Tenant ID"
//	@Param			body		body		request.CreateDatabase	true	"Database details"
//	@Success		202			{object}	model.Database
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/tenants/{tenantID}/databases [post]
func (h *Database) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateDatabase
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	shardID := req.ShardID
	database := &model.Database{
		ID:        platform.NewID(),
		TenantID:  &tenantID,
		Name:      req.Name,
		ShardID:   &shardID,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Create(r.Context(), database); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, database)
}

// Get godoc
//
//	@Summary		Get a database
//	@Tags			Databases
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Database ID"
//	@Success		200	{object}	model.Database
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/databases/{id} [get]
func (h *Database) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	database, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, database)
}

// Delete godoc
//
//	@Summary		Delete a database
//	@Tags			Databases
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Database ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/databases/{id} [delete]
func (h *Database) Delete(w http.ResponseWriter, r *http.Request) {
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

// Migrate godoc
//
//	@Summary		Migrate a database to a different shard
//	@Tags			Databases
//	@Security		ApiKeyAuth
//	@Param			id		path	string					true	"Database ID"
//	@Param			body	body	request.MigrateDatabase	true	"Target shard ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/databases/{id}/migrate [post]
func (h *Database) Migrate(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.MigrateDatabase
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Migrate(r.Context(), id, req.TargetShardID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// ReassignTenant godoc
//
//	@Summary		Reassign database to a different tenant
//	@Tags			Databases
//	@Security		ApiKeyAuth
//	@Param			id		path		string							true	"Database ID"
//	@Param			body	body		request.ReassignDatabaseTenant	true	"New tenant ID"
//	@Success		200		{object}	model.Database
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/databases/{id}/tenant [put]
func (h *Database) ReassignTenant(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.ReassignDatabaseTenant
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.ReassignTenant(r.Context(), id, req.TenantID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	database, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, database)
}
