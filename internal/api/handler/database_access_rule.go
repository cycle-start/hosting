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

type DatabaseAccessRule struct {
	svc    *core.DatabaseAccessRuleService
	dbSvc  *core.DatabaseService
}

func NewDatabaseAccessRule(svc *core.DatabaseAccessRuleService, dbSvc *core.DatabaseService) *DatabaseAccessRule {
	return &DatabaseAccessRule{svc: svc, dbSvc: dbSvc}
}

// ListByDatabase godoc
//
//	@Summary		List access rules for a database
//	@Description	Returns a paginated list of network access rules for the specified database. When rules exist, only connections from matching CIDRs are allowed.
//	@Tags			Database Access Rules
//	@Security		ApiKeyAuth
//	@Param			databaseID path string true "Database ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.DatabaseAccessRule}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/databases/{databaseID}/access-rules [get]
func (h *DatabaseAccessRule) ListByDatabase(w http.ResponseWriter, r *http.Request) {
	databaseID, err := request.RequireID(chi.URLParam(r, "databaseID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	rules, hasMore, err := h.svc.ListByDatabase(r.Context(), databaseID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(rules) > 0 {
		nextCursor = rules[len(rules)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, rules, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a database access rule
//	@Description	Adds a network access rule that restricts which source CIDRs can connect to a database. When rules exist, all MySQL users are recreated with host patterns matching the allowed CIDRs. When no rules exist, connections from any host are allowed. Async — returns 202.
//	@Tags			Database Access Rules
//	@Security		ApiKeyAuth
//	@Param			databaseID path string true "Database ID"
//	@Param			body body request.CreateDatabaseAccessRule true "Access rule details"
//	@Success		202 {object} model.DatabaseAccessRule
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/databases/{databaseID}/access-rules [post]
func (h *DatabaseAccessRule) Create(w http.ResponseWriter, r *http.Request) {
	databaseID, err := request.RequireID(chi.URLParam(r, "databaseID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateDatabaseAccessRule
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	rule := &model.DatabaseAccessRule{
		ID:          platform.NewID(),
		DatabaseID:  databaseID,
		CIDR:        req.CIDR,
		Description: req.Description,
		Status:      model.StatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := h.svc.Create(r.Context(), rule); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, rule)
}

// Get godoc
//
//	@Summary		Get a database access rule
//	@Description	Returns a single database access rule by ID.
//	@Tags			Database Access Rules
//	@Security		ApiKeyAuth
//	@Param			id path string true "Access rule ID"
//	@Success		200 {object} model.DatabaseAccessRule
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/database-access-rules/{id} [get]
func (h *DatabaseAccessRule) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	rule, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, rule)
}

// Delete godoc
//
//	@Summary		Delete a database access rule
//	@Description	Removes a database access rule. If this was the last rule, all MySQL users are recreated with host '%' (any host). Async — returns 202.
//	@Tags			Database Access Rules
//	@Security		ApiKeyAuth
//	@Param			id path string true "Access rule ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/database-access-rules/{id} [delete]
func (h *DatabaseAccessRule) Delete(w http.ResponseWriter, r *http.Request) {
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
//	@Summary		Retry a failed database access rule
//	@Description	Re-triggers the sync workflow for a database access rule in failed state.
//	@Tags			Database Access Rules
//	@Security		ApiKeyAuth
//	@Param			id path string true "Access rule ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/database-access-rules/{id}/retry [post]
func (h *DatabaseAccessRule) Retry(w http.ResponseWriter, r *http.Request) {
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
