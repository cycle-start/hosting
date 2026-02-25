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
//	@Summary		List regions
//	@Description	Returns a paginated list of regions. Regions are geographic areas (e.g. "osl-1") that contain clusters.
//	@Tags			Regions
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
		response.WriteServiceError(w, err)
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
//	@Summary		Create a region
//	@Description	Synchronously creates a region with a slug ID and optional JSON config. Returns 201 on success.
//	@Tags			Regions
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
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, region)
}

// Get godoc
//
//	@Summary		Get a region
//	@Description	Returns the details of a single region by ID.
//	@Tags			Regions
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
//	@Summary		Update a region
//	@Description	Synchronously performs a partial update of a region's name or config. Only provided fields are changed.
//	@Tags			Regions
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
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, region)
}

// Delete godoc
//
//	@Summary		Delete a region
//	@Description	Synchronously deletes a region. The region must have no clusters before it can be deleted.
//	@Tags			Regions
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
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

