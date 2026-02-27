package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type BackupHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewBackupHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *BackupHandler {
	return &BackupHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

type createBackupRequest struct {
	SubscriptionID string `json:"subscription_id" validate:"required"`
	Type           string `json:"type" validate:"required"`
}

// ListByCustomer fetches backups across all tenants the customer has subscriptions with the "backups" module.
//
//	@Summary      List backups by customer
//	@Description  Fetches backups across all tenants the customer has subscriptions with the "backups" module
//	@Tags         Backups
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.Backup
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/backups [get]
func (h *BackupHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	listByCustomerWithModule[hosting.Backup](w, r, h.customerSvc, h.subscriptionSvc, "backups", func(tid string) ([]hosting.Backup, error) {
		return h.hostingClient.ListBackupsByTenant(r.Context(), tid)
	})
}

// Create triggers a new backup for a tenant resolved from the subscription_id in the body.
//
//	@Summary      Create a backup
//	@Description  Triggers a new backup for a tenant resolved from the subscription_id in the body
//	@Tags         Backups
//	@Accept       json
//	@Produce      json
//	@Param        cid   path      string               true  "Customer ID"
//	@Param        body  body      createBackupRequest   true  "Backup creation payload"
//	@Success      201   {object}  hosting.Backup
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/backups [post]
func (h *BackupHandler) Create(w http.ResponseWriter, r *http.Request) {
	customerID, ok := authorizeCustomer(w, r, h.customerSvc, "cid")
	if !ok {
		return
	}

	subs, err := h.subscriptionSvc.ListByCustomerWithModule(r.Context(), customerID, "backups")
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var req createBackupRequest
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

	backup, err := h.hostingClient.CreateBackup(r.Context(), tenantID, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create backup")
		return
	}

	response.WriteJSON(w, http.StatusCreated, backup)
}

// Restore triggers a restore from a backup.
//
//	@Summary      Restore a backup
//	@Description  Triggers a restore from a backup
//	@Tags         Backups
//	@Produce      json
//	@Param        id   path      string  true  "Backup ID"
//	@Success      202  "Accepted"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /backups/{id}/restore [post]
func (h *BackupHandler) Restore(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	backup, err := h.hostingClient.GetBackup(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "backup not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, backup.TenantID) {
		return
	}

	if err := h.hostingClient.RestoreBackup(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to restore backup")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Delete removes a backup by ID.
//
//	@Summary      Delete a backup
//	@Description  Removes a backup by ID
//	@Tags         Backups
//	@Produce      json
//	@Param        id   path      string  true  "Backup ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /backups/{id} [delete]
func (h *BackupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	backup, err := h.hostingClient.GetBackup(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "backup not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, backup.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteBackup(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete backup")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
