package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
)

// Referenced in swag annotations.
var _ *model.ClusterLBAddress

type ClusterLBAddressHandler struct {
	service *core.ClusterLBAddressService
}

func NewClusterLBAddressHandler(service *core.ClusterLBAddressService) *ClusterLBAddressHandler {
	return &ClusterLBAddressHandler{service: service}
}

// List godoc
//
//	@Summary		List cluster load balancer addresses
//	@Description	Returns a paginated list of load balancer IP addresses for a cluster. These are the public IPs that DNS records point to.
//	@Tags			Cluster LB Addresses
//	@Security		ApiKeyAuth
//	@Param			clusterID	path		string	true	"Cluster ID"
//	@Param			cursor		query		string	false	"Pagination cursor"
//	@Param			limit		query		int		false	"Page size (default 50)"
//	@Success		200			{object}	response.PaginatedResponse{items=[]model.ClusterLBAddress}
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/clusters/{clusterID}/lb-addresses [get]
func (h *ClusterLBAddressHandler) List(w http.ResponseWriter, r *http.Request) {
	clusterID, err := request.RequireID(chi.URLParam(r, "clusterID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	addrs, hasMore, err := h.service.ListByCluster(r.Context(), clusterID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(addrs) > 0 {
		nextCursor = addrs[len(addrs)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, addrs, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a cluster load balancer address
//	@Description	Synchronously registers a load balancer IP address for a cluster. Address family (IPv4/IPv6) is auto-detected from the address. Returns 201 on success.
//	@Tags			Cluster LB Addresses
//	@Security		ApiKeyAuth
//	@Param			clusterID	path		string							true	"Cluster ID"
//	@Param			body		body		request.CreateClusterLBAddress	true	"LB Address details"
//	@Success		201			{object}	model.ClusterLBAddress
//	@Failure		400			{object}	response.ErrorResponse
//	@Failure		500			{object}	response.ErrorResponse
//	@Router			/clusters/{clusterID}/lb-addresses [post]
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
		response.WriteServiceError(w, err)
		return
	}
	response.WriteJSON(w, http.StatusCreated, addr)
}

// Get godoc
//
//	@Summary		Get a cluster load balancer address
//	@Description	Returns the details of a single load balancer address by ID.
//	@Tags			Cluster LB Addresses
//	@Security		ApiKeyAuth
//	@Param			id	path		string	true	"LB Address ID"
//	@Success		200	{object}	model.ClusterLBAddress
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		404	{object}	response.ErrorResponse
//	@Router			/lb-addresses/{id} [get]
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

// Delete godoc
//
//	@Summary		Delete a cluster load balancer address
//	@Description	Synchronously removes a load balancer address from a cluster.
//	@Tags			Cluster LB Addresses
//	@Security		ApiKeyAuth
//	@Param			id	path	string	true	"LB Address ID"
//	@Success		204
//	@Failure		400	{object}	response.ErrorResponse
//	@Failure		500	{object}	response.ErrorResponse
//	@Router			/lb-addresses/{id} [delete]
func (h *ClusterLBAddressHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
