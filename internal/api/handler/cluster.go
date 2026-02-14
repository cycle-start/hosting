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
		response.WriteError(w, http.StatusInternalServerError, err.Error())
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
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, cluster)
}

// Get godoc
//
//	@Summary		Get a cluster
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
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, cluster)
}

// Delete godoc
//
//	@Summary		Delete a cluster
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
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
