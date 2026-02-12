package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
)

type ClusterLBAddressHandler struct {
	service *core.ClusterLBAddressService
}

func NewClusterLBAddressHandler(service *core.ClusterLBAddressService) *ClusterLBAddressHandler {
	return &ClusterLBAddressHandler{service: service}
}

func (h *ClusterLBAddressHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	addrs, hasMore, err := h.service.ListByCluster(r.Context(), clusterID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(addrs) > 0 {
		nextCursor = addrs[len(addrs)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, addrs, nextCursor, hasMore)
}

func (h *ClusterLBAddressHandler) Create(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	var req request.CreateClusterLBAddress
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	addr, err := h.service.Create(r.Context(), clusterID, req.Address, req.Label)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusCreated, addr)
}

func (h *ClusterLBAddressHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	addr, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusOK, addr)
}

func (h *ClusterLBAddressHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
