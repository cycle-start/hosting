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

type Cluster struct {
	svc *core.ClusterService
}

func NewCluster(svc *core.ClusterService) *Cluster {
	return &Cluster{svc: svc}
}

// ListByRegion godoc
//
//	@Summary		List clusters in a region
//	@Description	Returns a paginated list of clusters belonging to a region. Clusters are groups of shards and nodes (e.g. "prod-1").
//	@Tags			Clusters
//	@Security		ApiKeyAuth
//	@Param			regionID	path		string	true	"Region ID"
//	@Param			limit		query		int		false	"Page size"	default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.Cluster}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/regions/{regionID}/clusters [get]
func (h *Cluster) ListByRegion(w http.ResponseWriter, r *http.Request) {
	regionID, err := request.RequireID(chi.URLParam(r, "regionID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	clusters, hasMore, err := h.svc.ListByRegion(r.Context(), regionID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(clusters) > 0 {
		nextCursor = clusters[len(clusters)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, clusters, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a cluster
//	@Description	Synchronously creates a cluster in a region with optional config and a spec defining shard topology. Returns 201 on success.
//	@Tags			Clusters
//	@Security		ApiKeyAuth
//	@Param			regionID	path		string					true	"Region ID"
//	@Param			body		body		request.CreateCluster	true	"Cluster data"
//	@Success		201			{object}	model.Cluster
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/regions/{regionID}/clusters [post]
func (h *Cluster) Create(w http.ResponseWriter, r *http.Request) {
	regionID, err := request.RequireID(chi.URLParam(r, "regionID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateCluster
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg := req.Config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}
	spec := req.Spec
	if spec == nil {
		spec = json.RawMessage(`{}`)
	}

	now := time.Now()
	cluster := &model.Cluster{
		ID:        platform.NewID(),
		RegionID:  regionID,
		Name:      req.Name,
		Config:    cfg,
		Status:    model.StatusPending,
		Spec:      spec,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Create(r.Context(), cluster); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, cluster)
}

// Get godoc
//
//	@Summary		Get a cluster
//	@Description	Returns cluster details including its spec and config.
//	@Tags			Clusters
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Cluster ID"
//	@Success		200	{object}	model.Cluster
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/clusters/{id} [get]
func (h *Cluster) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cluster, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, cluster)
}

// Update godoc
//
//	@Summary		Update a cluster
//	@Description	Synchronously performs a partial update of a cluster's name, status, config, or spec. Only provided fields are changed.
//	@Tags			Clusters
//	@Security		ApiKeyAuth
//	@Param			id		path		string					true	"Cluster ID"
//	@Param			body	body		request.UpdateCluster	true	"Cluster updates"
//	@Success		200		{object}	model.Cluster
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/clusters/{id} [put]
func (h *Cluster) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateCluster
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cluster, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Name != "" {
		cluster.Name = req.Name
	}
	if req.Status != "" {
		cluster.Status = req.Status
	}
	if req.Config != nil {
		cluster.Config = req.Config
	}
	if req.Spec != nil {
		cluster.Spec = req.Spec
	}

	if err := h.svc.Update(r.Context(), cluster); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, cluster)
}

// Delete godoc
//
//	@Summary		Delete a cluster
//	@Description	Synchronously deletes a cluster. The cluster must have no shards before it can be deleted.
//	@Tags			Clusters
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Cluster ID"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/clusters/{id} [delete]
func (h *Cluster) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListRuntimes godoc
//
//	@Summary		List runtimes for a cluster
//	@Description	Returns a paginated list of runtimes available in a cluster (e.g. php 8.5, node 22).
//	@Tags			Clusters
//	@Security	ApiKeyAuth
//	@Param		id		path	string	true	"Cluster ID"
//	@Param		limit	query	int		false	"Page size"	default(50)
//	@Param		cursor	query	string	false	"Pagination cursor"
//	@Success	200		{object}	response.PaginatedResponse{items=[]model.ClusterRuntime}
//	@Failure	400		{object}	response.ErrorResponse
//	@Failure	500		{object}	response.ErrorResponse
//	@Router		/clusters/{id}/runtimes [get]
func (h *Cluster) ListRuntimes(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	runtimes, hasMore, err := h.svc.ListRuntimes(r.Context(), id, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(runtimes) > 0 {
		nextCursor = runtimes[len(runtimes)-1].Runtime + "/" + runtimes[len(runtimes)-1].Version
	}
	response.WritePaginated(w, http.StatusOK, runtimes, nextCursor, hasMore)
}

// AddRuntime godoc
//
//	@Summary		Add a runtime to a cluster
//	@Description	Synchronously registers a runtime and version as available in a cluster. Returns 201 on success.
//	@Tags			Clusters
//	@Security	ApiKeyAuth
//	@Param		id		path		string						true	"Cluster ID"
//	@Param		body	body		request.AddClusterRuntime	true	"Runtime"
//	@Success	201		{object}	model.ClusterRuntime
//	@Failure	400		{object}	response.ErrorResponse
//	@Failure	500		{object}	response.ErrorResponse
//	@Router		/clusters/{id}/runtimes [post]
func (h *Cluster) AddRuntime(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.AddClusterRuntime
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	rt := &model.ClusterRuntime{
		ClusterID: id,
		Runtime:   req.Runtime,
		Version:   req.Version,
		Available: true,
	}

	if err := h.svc.AddRuntime(r.Context(), rt); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, rt)
}

// RemoveRuntime godoc
//
//	@Summary		Remove a runtime from a cluster
//	@Description	Synchronously removes a runtime and version from a cluster's available runtimes.
//	@Tags			Clusters
//	@Security	ApiKeyAuth
//	@Param		id		path	string							true	"Cluster ID"
//	@Param		body	body	request.RemoveClusterRuntime	true	"Runtime"
//	@Success	204
//	@Failure	400	{object}	response.ErrorResponse
//	@Failure	500	{object}	response.ErrorResponse
//	@Router		/clusters/{id}/runtimes [delete]
func (h *Cluster) RemoveRuntime(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.RemoveClusterRuntime
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.RemoveRuntime(r.Context(), id, req.Runtime, req.Version); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
