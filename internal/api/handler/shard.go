package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Shard struct {
	svc *core.ShardService
}

func NewShard(svc *core.ShardService) *Shard {
	return &Shard{svc: svc}
}

// ListByCluster godoc
//
//	@Summary		List shards in a cluster
//	@Tags			Shards
//	@Security		ApiKeyAuth
//	@Param			clusterID	path		string	true	"Cluster ID"
//	@Param			limit		query		int		false	"Page size"	default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.Shard}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/clusters/{clusterID}/shards [get]
func (h *Shard) ListByCluster(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	shards, hasMore, err := h.svc.ListByCluster(r.Context(), clusterID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(shards) > 0 {
		nextCursor = shards[len(shards)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, shards, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a shard
//	@Tags			Shards
//	@Security		ApiKeyAuth
//	@Param			clusterID	path		string				true	"Cluster ID"
//	@Param			body		body		request.CreateShard	true	"Shard data"
//	@Success		201			{object}	model.Shard
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/clusters/{clusterID}/shards [post]
func (h *Shard) Create(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateShard
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg := req.Config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}

	now := time.Now()
	shard := &model.Shard{
		ID:        platform.NewID(),
		ClusterID: clusterID,
		Name:      req.Name,
		Role:      req.Role,
		LBBackend: req.LBBackend,
		Config:    cfg,
		Status:    model.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Create(r.Context(), shard); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, shard)
}

// Get godoc
//
//	@Summary		Get a shard
//	@Tags			Shards
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Shard ID"
//	@Success		200	{object}	model.Shard
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/shards/{id} [get]
func (h *Shard) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	shard, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, shard)
}

// Update godoc
//
//	@Summary		Update a shard
//	@Tags			Shards
//	@Security		ApiKeyAuth
//	@Param			id		path		string				true	"Shard ID"
//	@Param			body	body		request.UpdateShard	true	"Shard updates"
//	@Success		200		{object}	model.Shard
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/shards/{id} [put]
func (h *Shard) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateShard
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	shard, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.LBBackend != "" {
		shard.LBBackend = req.LBBackend
	}
	if req.Config != nil {
		shard.Config = req.Config
	}
	if req.Status != "" {
		shard.Status = req.Status
	}

	if err := h.svc.Update(r.Context(), shard); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, shard)
}

// Converge godoc
//
//	@Summary		Trigger shard convergence
//	@Tags			Shards
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Shard ID"
//	@Success		202	{object}	map[string]string
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/shards/{id}/converge [post]
func (h *Shard) Converge(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	_, err = h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := h.svc.Converge(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, map[string]string{"status": "converging"})
}

// Retry godoc
//
//	@Summary		Retry a failed shard convergence
//	@Tags			Shards
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Shard ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/shards/{id}/retry [post]
func (h *Shard) Retry(w http.ResponseWriter, r *http.Request) {
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

// Delete godoc
//
//	@Summary		Delete a shard
//	@Tags			Shards
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Shard ID"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/shards/{id} [delete]
func (h *Shard) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
