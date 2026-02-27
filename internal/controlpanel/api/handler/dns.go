package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type DNSHandler struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewDNSHandler(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *DNSHandler {
	return &DNSHandler{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

// ListByCustomer fetches all DNS zones and filters to those belonging to tenants the customer has access to.
//
//	@Summary      List DNS zones by customer
//	@Description  Fetches all DNS zones and filters to those belonging to tenants the customer has access to
//	@Tags         DNS Zones
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.Zone
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/dns-zones [get]
func (h *DNSHandler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	customerID, ok := authorizeCustomer(w, r, h.customerSvc, "cid")
	if !ok {
		return
	}

	subs, err := h.subscriptionSvc.ListByCustomerWithModule(r.Context(), customerID, "dns")
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	tenantSet := make(map[string]bool)
	for _, sub := range subs {
		tenantSet[sub.TenantID] = true
	}

	if len(tenantSet) == 0 {
		response.WriteJSON(w, http.StatusOK, map[string]any{"items": []hosting.Zone{}})
		return
	}

	allZones, err := h.hostingClient.ListAllZones(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch zones")
		return
	}

	var filtered []hosting.Zone
	for _, z := range allZones {
		if tenantSet[z.TenantID] {
			filtered = append(filtered, z)
		}
	}
	if filtered == nil {
		filtered = []hosting.Zone{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": filtered})
}

// Get returns a single DNS zone by ID, with authorization check.
//
//	@Summary      Get a DNS zone
//	@Description  Returns a single DNS zone by ID, with authorization check
//	@Tags         DNS Zones
//	@Produce      json
//	@Param        id   path      string  true  "DNS Zone ID"
//	@Success      200  {object}  hosting.Zone
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /dns-zones/{id} [get]
func (h *DNSHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.hostingClient.GetZone(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "zone not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, zone.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, zone)
}

// Update updates a DNS zone.
//
//	@Summary      Update a DNS zone
//	@Description  Updates a DNS zone's settings
//	@Tags         DNS Zones
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "DNS Zone ID"
//	@Param        body  body      object  true  "Zone settings"
//	@Success      200   {object}  hosting.Zone
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /dns-zones/{id} [put]
func (h *DNSHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.hostingClient.GetZone(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "zone not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, zone.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.hostingClient.UpdateZone(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update zone")
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

// Delete removes a DNS zone by ID.
//
//	@Summary      Delete a DNS zone
//	@Description  Removes a DNS zone by ID
//	@Tags         DNS Zones
//	@Produce      json
//	@Param        id   path      string  true  "DNS Zone ID"
//	@Success      204  "No Content"
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /dns-zones/{id} [delete]
func (h *DNSHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.hostingClient.GetZone(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "zone not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, zone.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteZone(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete zone")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListRecords returns all records for a DNS zone.
//
//	@Summary      List DNS zone records
//	@Description  Returns all records for a DNS zone
//	@Tags         DNS Zones
//	@Produce      json
//	@Param        id   path      string  true  "DNS Zone ID"
//	@Success      200  {object}  map[string][]hosting.ZoneRecord
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /dns-zones/{id}/records [get]
func (h *DNSHandler) ListRecords(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.hostingClient.GetZone(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "zone not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, zone.TenantID) {
		return
	}

	records, err := h.hostingClient.ListZoneRecords(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch zone records")
		return
	}
	if records == nil {
		records = []hosting.ZoneRecord{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": records})
}

// CreateRecord creates a record in a DNS zone.
//
//	@Summary      Create DNS zone record
//	@Description  Creates a new record in a DNS zone
//	@Tags         DNS Zones
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "DNS Zone ID"
//	@Param        body  body      object  true  "Record data"
//	@Success      201   {object}  hosting.ZoneRecord
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /dns-zones/{id}/records [post]
func (h *DNSHandler) CreateRecord(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.hostingClient.GetZone(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "zone not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, zone.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	record, err := h.hostingClient.CreateZoneRecord(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create zone record")
		return
	}

	response.WriteJSON(w, http.StatusCreated, record)
}

// UpdateRecord updates a record in a DNS zone.
//
//	@Summary      Update DNS zone record
//	@Description  Updates a record in a DNS zone
//	@Tags         DNS Zones
//	@Accept       json
//	@Produce      json
//	@Param        id        path      string  true  "DNS Zone ID"
//	@Param        recordId  path      string  true  "Record ID"
//	@Param        body      body      object  true  "Record data"
//	@Success      200       {object}  hosting.ZoneRecord
//	@Failure      400       {object}  response.ErrorResponse
//	@Failure      403       {object}  response.ErrorResponse
//	@Failure      404       {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /dns-zones/{id}/records/{recordId} [put]
func (h *DNSHandler) UpdateRecord(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.hostingClient.GetZone(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "zone not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, zone.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	recordID := chi.URLParam(r, "recordId")
	record, err := h.hostingClient.UpdateZoneRecord(r.Context(), recordID, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update zone record")
		return
	}

	response.WriteJSON(w, http.StatusOK, record)
}

// DeleteRecord deletes a record from a DNS zone.
//
//	@Summary      Delete DNS zone record
//	@Description  Deletes a record from a DNS zone
//	@Tags         DNS Zones
//	@Produce      json
//	@Param        id        path      string  true  "DNS Zone ID"
//	@Param        recordId  path      string  true  "Record ID"
//	@Success      204       "No Content"
//	@Failure      400       {object}  response.ErrorResponse
//	@Failure      403       {object}  response.ErrorResponse
//	@Failure      404       {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /dns-zones/{id}/records/{recordId} [delete]
func (h *DNSHandler) DeleteRecord(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	zone, err := h.hostingClient.GetZone(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "zone not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, zone.TenantID) {
		return
	}

	recordID := chi.URLParam(r, "recordId")
	if err := h.hostingClient.DeleteZoneRecord(r.Context(), recordID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete zone record")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
