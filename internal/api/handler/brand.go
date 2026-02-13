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
