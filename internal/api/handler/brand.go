package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/go-chi/chi/v5"
)

type Brand struct {
	svc *core.BrandService
}

func NewBrand(svc *core.BrandService) *Brand {
	return &Brand{svc: svc}
}

// List godoc
//
//	@Summary		List brands
//	@Description	Returns a paginated list of brands. Brands are the top-level isolation boundary — each brand defines its own NS hostnames, hostmaster email, base domain, and cluster access. All tenants, zones, and FQDNs are scoped to a brand.
//	@Tags			Brands
//	@Security		ApiKeyAuth
//	@Param			search query string false "Filter brands by name (case-insensitive substring match)"
//	@Param			status query string false "Filter by status"
//	@Param			sort query string false "Sort field" default(created_at)
//	@Param			order query string false "Sort order (asc/desc)" default(asc)
//	@Param			limit query int false "Page size (max 200)" default(50)
//	@Param			cursor query string false "Pagination cursor (brand ID from previous page)"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.Brand}
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/brands [get]
func (h *Brand) List(w http.ResponseWriter, r *http.Request) {
	params := request.ParseListParams(r, "created_at")

	brands, hasMore, err := h.svc.List(r.Context(), params)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(brands) > 0 {
		nextCursor = brands[len(brands)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, brands, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a brand
//	@Description	Creates a new brand with the given slug ID. The brand ID is caller-provided and must be a unique slug (e.g. "acme", "myhost"). Brands define NS hostnames, hostmaster email, and base domain used for DNS zone SOA records and tenant hostname generation.
//	@Tags			Brands
//	@Security		ApiKeyAuth
//	@Param			body body request.CreateBrand true "Brand details"
//	@Success		201 {object} model.Brand
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/brands [post]
func (h *Brand) Create(w http.ResponseWriter, r *http.Request) {
	var req request.CreateBrand
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	brand := &model.Brand{
		ID:              req.ID,
		Name:            req.Name,
		BaseHostname:    req.BaseHostname,
		PrimaryNS:       req.PrimaryNS,
		SecondaryNS:     req.SecondaryNS,
		HostmasterEmail: req.HostmasterEmail,
		Status:          model.StatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := h.svc.Create(r.Context(), brand); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, brand)
}

// Get godoc
//
//	@Summary		Get a brand
//	@Description	Returns a single brand by its slug ID, including its NS configuration and status.
//	@Tags			Brands
//	@Security		ApiKeyAuth
//	@Param			id path string true "Brand ID (slug)"
//	@Success		200 {object} model.Brand
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/brands/{id} [get]
func (h *Brand) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	brand, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, brand)
}

// Update godoc
//
//	@Summary		Update a brand
//	@Description	Partially updates a brand. Only provided fields are changed. Updating NS hostnames or hostmaster email affects SOA records for all zones under this brand on next convergence.
//	@Tags			Brands
//	@Security		ApiKeyAuth
//	@Param			id path string true "Brand ID (slug)"
//	@Param			body body request.UpdateBrand true "Fields to update (partial)"
//	@Success		200 {object} model.Brand
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/brands/{id} [put]
func (h *Brand) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateBrand
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	brand, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Name != nil {
		brand.Name = *req.Name
	}
	if req.BaseHostname != nil {
		brand.BaseHostname = *req.BaseHostname
	}
	if req.PrimaryNS != nil {
		brand.PrimaryNS = *req.PrimaryNS
	}
	if req.SecondaryNS != nil {
		brand.SecondaryNS = *req.SecondaryNS
	}
	if req.HostmasterEmail != nil {
		brand.HostmasterEmail = *req.HostmasterEmail
	}

	if err := h.svc.Update(r.Context(), brand); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, brand)
}

// Delete godoc
//
//	@Summary		Delete a brand
//	@Description	Deletes a brand. The brand must have no tenants or zones associated with it. This is a permanent, irreversible operation.
//	@Tags			Brands
//	@Security		ApiKeyAuth
//	@Param			id path string true "Brand ID (slug)"
//	@Success		200
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/brands/{id} [delete]
func (h *Brand) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
}

// ListClusters godoc
//
//	@Summary		List allowed clusters for a brand
//	@Description	Returns the list of cluster IDs that this brand is allowed to provision tenants on. An empty list means no restriction (all clusters allowed).
//	@Tags			Brands
//	@Security		ApiKeyAuth
//	@Param			id path string true "Brand ID (slug)"
//	@Success		200 {object} map[string][]string
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/brands/{id}/clusters [get]
func (h *Brand) ListClusters(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	clusterIDs, err := h.svc.ListClusters(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if clusterIDs == nil {
		clusterIDs = []string{}
	}

	response.WriteJSON(w, http.StatusOK, map[string][]string{"cluster_ids": clusterIDs})
}

// SetClusters godoc
//
//	@Summary		Set allowed clusters for a brand
//	@Description	Replaces the list of clusters this brand is allowed to use for tenant provisioning. Pass an empty array to remove all restrictions. Tenant creation will be rejected if the target cluster is not in this list (when non-empty).
//	@Tags			Brands
//	@Security		ApiKeyAuth
//	@Param			id path string true "Brand ID (slug)"
//	@Param			body body request.SetBrandClusters true "Cluster IDs"
//	@Success		200 {object} map[string][]string
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/brands/{id}/clusters [put]
func (h *Brand) SetClusters(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.SetBrandClusters
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.SetClusters(r.Context(), id, req.ClusterIDs); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string][]string{"cluster_ids": req.ClusterIDs})
}
