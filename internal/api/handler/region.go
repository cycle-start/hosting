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

type Region struct {
	svc *core.RegionService
}

func NewRegion(svc *core.RegionService) *Region {
	return &Region{svc: svc}
}

// List godoc
//
//	@Summary	List regions
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		limit	query	int		false	"Page size"	default(50)
//	@Param		cursor	query	string	false	"Pagination cursor"
//	@Success	200		{object}	response.PaginatedResponse{items=[]model.Region}
//	@Failure	500		{object}	response.ErrorResponse
//	@Router		/regions [get]
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

// Create godoc
//
//	@Summary	Create a region
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		body	body		request.CreateRegion	true	"Region"
//	@Success	201		{object}	model.Region
//	@Failure	400		{object}	response.ErrorResponse
//	@Failure	500		{object}	response.ErrorResponse
//	@Router		/regions [post]
func (h *Region) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateRegion
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
		ID:        platform.NewID(),
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

// Get godoc
//
//	@Summary	Get a region
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		id	path		string	true	"Region ID"
//	@Success	200	{object}	model.Region
//	@Failure	400	{object}	response.ErrorResponse
//	@Failure	404	{object}	response.ErrorResponse
//	@Router		/regions/{id} [get]
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

// Update godoc
//
//	@Summary	Update a region
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		id		path		string				true	"Region ID"
//	@Param		body	body		request.UpdateRegion	true	"Region"
//	@Success	200		{object}	model.Region
//	@Failure	400		{object}	response.ErrorResponse
//	@Failure	404		{object}	response.ErrorResponse
//	@Failure	500		{object}	response.ErrorResponse
//	@Router		/regions/{id} [put]
func (h *Region) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateRegion
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

// Delete godoc
//
//	@Summary	Delete a region
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		id	path	string	true	"Region ID"
//	@Success	204
//	@Failure	400	{object}	response.ErrorResponse
//	@Failure	500	{object}	response.ErrorResponse
//	@Router		/regions/{id} [delete]
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

// ListRuntimes godoc
//
//	@Summary	List runtimes for a region
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		id		path	string	true	"Region ID"
//	@Param		limit	query	int		false	"Page size"	default(50)
//	@Param		cursor	query	string	false	"Pagination cursor"
//	@Success	200		{object}	response.PaginatedResponse{items=[]model.RegionRuntime}
//	@Failure	400		{object}	response.ErrorResponse
//	@Failure	500		{object}	response.ErrorResponse
//	@Router		/regions/{id}/runtimes [get]
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

// AddRuntime godoc
//
//	@Summary	Add a runtime to a region
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		id		path		string					true	"Region ID"
//	@Param		body	body		request.AddRegionRuntime	true	"Runtime"
//	@Success	201		{object}	model.RegionRuntime
//	@Failure	400		{object}	response.ErrorResponse
//	@Failure	500		{object}	response.ErrorResponse
//	@Router		/regions/{id}/runtimes [post]
func (h *Region) AddRuntime(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.AddRegionRuntime
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

// RemoveRuntime godoc
//
//	@Summary	Remove a runtime from a region
//	@Tags		Regions
//	@Security	ApiKeyAuth
//	@Param		id		path	string						true	"Region ID"
//	@Param		body	body	request.RemoveRegionRuntime	true	"Runtime"
//	@Success	204
//	@Failure	400	{object}	response.ErrorResponse
//	@Failure	500	{object}	response.ErrorResponse
//	@Router		/regions/{id}/runtimes [delete]
func (h *Region) RemoveRuntime(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.RemoveRegionRuntime
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
