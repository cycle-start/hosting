package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/go-chi/chi/v5"
)

type Cluster struct {
	svc *core.ClusterService
}

func NewCluster(svc *core.ClusterService) *Cluster {
	return &Cluster{svc: svc}
}

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

func (h *Cluster) Create(w http.ResponseWriter, r *http.Request) {
	regionID, err := request.RequireID(chi.URLParam(r, "regionID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		ID     string          `json:"id" validate:"required,slug"`
		Name   string          `json:"name" validate:"required,slug"`
		Config json.RawMessage `json:"config"`
		Spec   json.RawMessage `json:"spec"`
	}
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
		ID:        req.ID,
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

func (h *Cluster) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
		Spec   json.RawMessage `json:"spec"`
	}
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

