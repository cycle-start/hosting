package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type SSHKeyHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewSSHKeyHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *SSHKeyHandler {
	return &SSHKeyHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

type createSSHKeyRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	Name           string `json:"name" validate:"required"`
	PublicKey      string `json:"public_key" validate:"required"`
}

// ListByCustomer fetches SSH keys across all tenants the customer has subscriptions with the "ssh_keys" module.
//
//	@Summary      List SSH keys by customer
//	@Description  Fetches SSH keys across all tenants the customer has subscriptions with the "ssh_keys" module
//	@Tags         SSH Keys
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.SSHKey
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/ssh-keys [get]
func (h *SSHKeyHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	listByCustomerWithModule[hosting.SSHKey](w, r, h.customerSvc, h.subscriptionSvc, "ssh_keys", func(tid string) ([]hosting.SSHKey, error) {
		return h.hostingClient.ListSSHKeysByTenant(r.Context(), tid)
	})
}

// Create creates an SSH key for a tenant resolved from the subscription_id in the body.
//
//	@Summary      Create an SSH key
//	@Description  Creates an SSH key for a tenant resolved from the subscription_id in the body
//	@Tags         SSH Keys
//	@Accept       json
//	@Produce      json
//	@Param        cid   path      string              true  "Customer ID"
//	@Param        body  body      createSSHKeyRequest  true  "SSH key creation payload"
//	@Success      201   {object}  hosting.SSHKey
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/ssh-keys [post]
func (h *SSHKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	customerID, ok := authorizeCustomer(w, r, h.customerSvc, "cid")
	if !ok {
		return
	}

	subs, err := h.subscriptionSvc.ListByCustomerWithModule(r.Context(), customerID, "ssh_keys")
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var req createSSHKeyRequest
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

	key, err := h.hostingClient.CreateSSHKey(r.Context(), tenantID, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create ssh key")
		return
	}

	response.WriteJSON(w, http.StatusCreated, key)
}

// Delete removes an SSH key by ID.
//
//	@Summary      Delete an SSH key
//	@Description  Removes an SSH key by ID
//	@Tags         SSH Keys
//	@Produce      json
//	@Param        id   path      string  true  "SSH Key ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /ssh-keys/{id} [delete]
func (h *SSHKeyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	key, err := h.hostingClient.GetSSHKey(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "ssh key not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, key.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteSSHKey(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete ssh key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
