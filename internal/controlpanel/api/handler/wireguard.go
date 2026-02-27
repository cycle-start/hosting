package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type WireGuardHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewWireGuardHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *WireGuardHandler {
	return &WireGuardHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

type createWireGuardPeerRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	Name           string `json:"name" validate:"required"`
}

// ListByCustomer fetches WireGuard peers across all tenants the customer has subscriptions with the "wireguard" module.
//
//	@Summary      List WireGuard peers by customer
//	@Description  Fetches WireGuard peers across all tenants the customer has subscriptions with the "wireguard" module
//	@Tags         WireGuard
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.WireGuardPeer
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/wireguard [get]
func (h *WireGuardHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	listByCustomerWithModule[hosting.WireGuardPeer](w, r, h.customerSvc, h.subscriptionSvc, "wireguard", func(tid string) ([]hosting.WireGuardPeer, error) {
		return h.hostingClient.ListWireGuardPeersByTenant(r.Context(), tid)
	})
}

// Get returns a single WireGuard peer by ID, with authorization check.
//
//	@Summary      Get a WireGuard peer
//	@Description  Returns a single WireGuard peer by ID, with authorization check
//	@Tags         WireGuard
//	@Produce      json
//	@Param        id   path      string  true  "WireGuard Peer ID"
//	@Success      200  {object}  hosting.WireGuardPeer
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /wireguard/{id} [get]
func (h *WireGuardHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	peer, err := h.hostingClient.GetWireGuardPeer(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "wireguard peer not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, peer.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, peer)
}

// Create creates a WireGuard peer for a tenant resolved from the subscription_id in the body.
//
//	@Summary      Create a WireGuard peer
//	@Description  Creates a WireGuard peer for a tenant resolved from the subscription_id in the body
//	@Tags         WireGuard
//	@Accept       json
//	@Produce      json
//	@Param        cid   path      string                       true  "Customer ID"
//	@Param        body  body      createWireGuardPeerRequest   true  "WireGuard peer creation payload"
//	@Success      202   {object}  hosting.WireGuardPeerCreateResult
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/wireguard [post]
func (h *WireGuardHandler) Create(w http.ResponseWriter, r *http.Request) {
	customerID, ok := authorizeCustomer(w, r, h.customerSvc, "cid")
	if !ok {
		return
	}

	subs, err := h.subscriptionSvc.ListByCustomerWithModule(r.Context(), customerID, "wireguard")
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var req createWireGuardPeerRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Find the matching subscription to resolve the tenant ID
	var tenantID string
	for _, sub := range subs {
		if sub.ID == req.SubscriptionID {
			tenantID = sub.TenantID
			break
		}
	}
	if tenantID == "" {
		response.WriteError(w, http.StatusForbidden, "no access to this subscription")
		return
	}

	result, err := h.hostingClient.CreateWireGuardPeer(r.Context(), tenantID, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create wireguard peer")
		return
	}

	response.WriteJSON(w, http.StatusAccepted, result)
}

// Delete removes a WireGuard peer by ID.
//
//	@Summary      Delete a WireGuard peer
//	@Description  Removes a WireGuard peer by ID
//	@Tags         WireGuard
//	@Produce      json
//	@Param        id   path      string  true  "WireGuard Peer ID"
//	@Success      202  "Accepted"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /wireguard/{id} [delete]
func (h *WireGuardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	peer, err := h.hostingClient.GetWireGuardPeer(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "wireguard peer not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, peer.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteWireGuardPeer(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete wireguard peer")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
