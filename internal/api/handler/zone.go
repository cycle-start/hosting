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

type Zone struct {
	svc      *core.ZoneService
	services *core.Services
}

func NewZone(services *core.Services) *Zone {
	return &Zone{svc: services.Zone, services: services}
}

// List godoc
//
//	@Summary		List zones
//	@Description	Returns a paginated list of all DNS zones across all brands. Supports filtering by search term and status, and sorting by any column.
//	@Tags			Zones
//	@Security		ApiKeyAuth
//	@Param			limit		query		int		false	"Page size"						default(50)
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Param			search		query		string	false	"Search term"
//	@Param			status		query		string	false	"Filter by status"
//	@Param			sort		query		string	false	"Sort field"					default(created_at)
//	@Param			order		query		string	false	"Sort order (asc or desc)"		default(asc)
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.Zone}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/zones [get]
func (h *Zone) List(w http.ResponseWriter, r *http.Request) {
	params := request.ParseListParams(r, "created_at")

	zones, hasMore, err := h.svc.List(r.Context(), params)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(zones) > 0 {
		nextCursor = zones[len(zones)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, zones, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a zone
//	@Description	Creates a DNS zone (e.g. "example.com"). Requires brand_id and region_id; if tenant_id is provided, the brand is derived from the tenant. Returns 202 and triggers a Temporal workflow to create the zone in the brand's PowerDNS database with SOA and NS records.
//	@Tags			Zones
//	@Security		ApiKeyAuth
//	@Param			body	body		request.CreateZone	true	"Zone details"
//	@Success		202		{object}	model.Zone
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/zones [post]
func (h *Zone) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateZone
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Derive brand_id: from tenant if provided, otherwise from request.
	brandID := req.BrandID
	if req.TenantID != nil && *req.TenantID != "" {
		tenant, err := h.services.Tenant.GetByID(r.Context(), *req.TenantID)
		if err != nil {
			response.WriteError(w, http.StatusBadRequest, "invalid tenant_id: "+err.Error())
			return
		}
		brandID = tenant.BrandID
	}
	if brandID == "" {
		response.WriteError(w, http.StatusBadRequest, "brand_id is required when tenant_id is not provided")
		return
	}

	now := time.Now()
	zone := &model.Zone{
		ID:        platform.NewID(),
		BrandID:   brandID,
		TenantID:  req.TenantID,
		Name:      req.Name,
		RegionID:  req.RegionID,
		Status:    model.StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Create(r.Context(), zone); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, zone)
}

// Get godoc
//
//	@Summary		Get a zone
//	@Description	Returns the details of a single DNS zone, including its region name.
//	@Tags			Zones
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"Zone ID"
//	@Success		200	{object}	model.Zone
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/zones/{id} [get]
func (h *Zone) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, zone)
}

// Update godoc
//
//	@Summary		Update a zone
//	@Description	Updates a DNS zone. Currently only supports changing the tenant_id association. This is a synchronous operation.
//	@Tags			Zones
//	@Security		ApiKeyAuth
//	@Param			id		path		string				true	"Zone ID"
//	@Param			body	body		request.UpdateZone	true	"Zone updates"
//	@Success		200		{object}	model.Zone
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		404		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/zones/{id} [put]
func (h *Zone) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateZone
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	zone.TenantID = req.TenantID

	if err := h.svc.Update(r.Context(), zone); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, zone)
}

// Delete godoc
//
//	@Summary		Delete a zone
//	@Description	Deletes a DNS zone and all its records from PowerDNS. Returns 202 and triggers a Temporal workflow.
//	@Tags			Zones
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"Zone ID"
//	@Success		202
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/zones/{id} [delete]
func (h *Zone) Delete(w http.ResponseWriter, r *http.Request) {
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

// ReassignTenant godoc
//
//	@Summary		Reassign zone to a different tenant
//	@Description	Changes the tenant ownership of a DNS zone without affecting the zone data in PowerDNS. This is a synchronous operation. Pass null tenant_id to detach the zone from its current tenant.
//	@Tags			Zones
//	@Security		ApiKeyAuth
//	@Param			id		path		string						true	"Zone ID"
//	@Param			body	body		request.ReassignZoneTenant	true	"New tenant ID"
//	@Success		200		{object}	model.Zone
//	@Failure		400		{object}	response.ErrorResponse
//	@Failure		500		{object}	response.ErrorResponse
//	@Router			/zones/{id}/tenant [put]
func (h *Zone) ReassignTenant(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.ReassignZoneTenant
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.ReassignTenant(r.Context(), id, req.TenantID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	zone, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, zone)
}

// Retry godoc
//
//	@Summary		Retry a failed zone
//	@Description	Re-triggers the provisioning workflow for a DNS zone that is in a failed state. Returns 202 and starts a new Temporal workflow.
//	@Tags			Zones
//	@Security		ApiKeyAuth
//	@Param			id path string true "Zone ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/zones/{id}/retry [post]
func (h *Zone) Retry(w http.ResponseWriter, r *http.Request) {
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
