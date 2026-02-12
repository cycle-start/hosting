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

type Region struct {
	svc *core.RegionService
}

func NewRegion(svc *core.RegionService) *Region {
	return &Region{svc: svc}
}

func (h *Region) List(w http.ResponseWriter, r *http.Request) {
	pg := request.ParsePagination(r)

	regions, hasMore, err := h.svc.List(r.Context(), pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(regions) > 0 {
		nextCursor = regions[len(regions)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, regions, nextCursor, hasMore)
}

func (h *Region) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string          `json:"id" validate:"required,slug"`
		Name   string          `json:"name" validate:"required,slug"`
		Config json.RawMessage `json:"config"`
	}
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cfg := req.Config
	if cfg == nil {
		cfg = json.RawMessage(`{}`)
	}

	now := time.Now()
	region := &model.Region{
		ID:        req.ID,
		Name:      req.Name,
		Config:    cfg,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Create(r.Context(), region); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, region)
}

func (h *Region) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	region, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, region)
}

func (h *Region) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	region, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Name != "" {
		region.Name = req.Name
	}
	if req.Config != nil {
		region.Config = req.Config
	}

	if err := h.svc.Update(r.Context(), region); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, region)
}

func (h *Region) Delete(w http.ResponseWriter, r *http.Request) {
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

func (h *Region) ListRuntimes(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	runtimes, hasMore, err := h.svc.ListRuntimes(r.Context(), id, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(runtimes) > 0 {
		nextCursor = runtimes[len(runtimes)-1].Runtime + "/" + runtimes[len(runtimes)-1].Version
	}
	response.WritePaginated(w, http.StatusOK, runtimes, nextCursor, hasMore)
}

func (h *Region) AddRuntime(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		Runtime string `json:"runtime" validate:"required"`
		Version string `json:"version" validate:"required"`
	}
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	rt := &model.RegionRuntime{
		RegionID:  id,
		Runtime:   req.Runtime,
		Version:   req.Version,
		Available: true,
	}

	if err := h.svc.AddRuntime(r.Context(), rt); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, rt)
}

func (h *Region) RemoveRuntime(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req struct {
		Runtime string `json:"runtime" validate:"required"`
		Version string `json:"version" validate:"required"`
	}
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.RemoveRuntime(r.Context(), id, req.Runtime, req.Version); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
