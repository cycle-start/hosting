package handler

import (
	"net/http"
	"sync"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type Webroot struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	hostingClient   *hosting.Client
}

func NewWebroot(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, hostingClient *hosting.Client) *Webroot {
	return &Webroot{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		hostingClient:   hostingClient,
	}
}

// ListByCustomer fetches webroots across all tenants the customer has subscriptions with the "webroots" module.
//
//	@Summary      List webroots by customer
//	@Description  Fetches webroots across all tenants the customer has access to
//	@Tags         Webroots
//	@Produce      json
//	@Param        cid  path      string  true  "Customer ID"
//	@Success      200  {object}  map[string][]hosting.Webroot
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      401  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{cid}/webroots [get]
func (h *Webroot) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	customerID, err := request.RequireID(chi.URLParam(r, "cid"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	hasAccess, err := h.customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}
	if !hasAccess {
		response.WriteError(w, http.StatusForbidden, "no access to this customer")
		return
	}

	// Get subscriptions with "webroots" module
	subs, err := h.subscriptionSvc.ListByCustomerWithModule(r.Context(), customerID, "webroots")
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	// Collect tenant IDs
	var tenantIDs []string
	for _, sub := range subs {
		tenantIDs = append(tenantIDs, sub.TenantID)
	}

	if len(tenantIDs) == 0 {
		response.WriteJSON(w, http.StatusOK, map[string]any{"items": []hosting.Webroot{}})
		return
	}

	// Fan out concurrent requests to hosting API
	type result struct {
		webroots []hosting.Webroot
		err      error
	}
	results := make([]result, len(tenantIDs))
	var wg sync.WaitGroup

	for i, tid := range tenantIDs {
		wg.Add(1)
		go func(idx int, tenantID string) {
			defer wg.Done()
			webroots, err := h.hostingClient.ListWebrootsByTenant(r.Context(), tenantID)
			results[idx] = result{webroots: webroots, err: err}
		}(i, tid)
	}
	wg.Wait()

	// Merge results, tolerate partial failures
	var allWebroots []hosting.Webroot
	for _, res := range results {
		if res.err == nil {
			allWebroots = append(allWebroots, res.webroots...)
		}
	}
	if allWebroots == nil {
		allWebroots = []hosting.Webroot{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": allWebroots})
}

// Get returns a single webroot by ID, with authorization check.
//
//	@Summary      Get webroot
//	@Description  Returns a single webroot by ID
//	@Tags         Webroots
//	@Produce      json
//	@Param        id   path      string  true  "Webroot ID"
//	@Success      200  {object}  hosting.Webroot
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id} [get]
func (h *Webroot) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch webroot from hosting API
	webroot, err := h.hostingClient.GetWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	// Map tenant_id → customer_id via subscription
	customerID, err := h.customerSvc.GetCustomerIDByTenant(r.Context(), webroot.TenantID)
	if err != nil {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return
	}

	// Check user has access to the customer
	hasAccess, err := h.customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil || !hasAccess {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return
	}

	response.WriteJSON(w, http.StatusOK, webroot)
}

// ListFQDNs returns all FQDNs for a webroot.
//
//	@Summary      List webroot FQDNs
//	@Description  Returns all hostnames attached to a webroot
//	@Tags         Webroots
//	@Produce      json
//	@Param        id   path      string  true  "Webroot ID"
//	@Success      200  {object}  map[string][]hosting.FQDN
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/fqdns [get]
func (h *Webroot) ListFQDNs(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	fqdns, err := h.hostingClient.ListFQDNsByWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch FQDNs")
		return
	}
	if fqdns == nil {
		fqdns = []hosting.FQDN{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": fqdns})
}

// ListAvailableFQDNs returns all tenant FQDNs not attached to any webroot.
//
//	@Summary      List available FQDNs
//	@Description  Returns all tenant FQDNs not currently attached to any webroot
//	@Tags         Webroots
//	@Produce      json
//	@Param        id   path      string  true  "Webroot ID"
//	@Success      200  {object}  map[string][]hosting.FQDN
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/available-fqdns [get]
func (h *Webroot) ListAvailableFQDNs(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	fqdns, err := h.hostingClient.ListFQDNsByTenant(r.Context(), webroot.TenantID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch FQDNs")
		return
	}

	// Filter to only unattached FQDNs
	var available []hosting.FQDN
	for _, f := range fqdns {
		if f.WebrootID == nil {
			available = append(available, f)
		}
	}
	if available == nil {
		available = []hosting.FQDN{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": available})
}

// AttachFQDN sets a hostname's webroot_id to attach it to a webroot.
//
//	@Summary      Attach FQDN to webroot
//	@Description  Attaches a hostname to a webroot by setting its webroot_id
//	@Tags         Webroots
//	@Produce      json
//	@Param        id      path      string  true  "Webroot ID"
//	@Param        fqdnId  path      string  true  "FQDN ID"
//	@Success      200     {object}  hosting.FQDN
//	@Failure      400     {object}  response.ErrorResponse
//	@Failure      403     {object}  response.ErrorResponse
//	@Failure      404     {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/fqdns/{fqdnId}/attach [post]
func (h *Webroot) AttachFQDN(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fqdnID, err := request.RequireID(chi.URLParam(r, "fqdnId"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	// Verify the FQDN belongs to the same tenant
	fqdn, err := h.hostingClient.GetFQDN(r.Context(), fqdnID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "hostname not found")
		return
	}
	if fqdn.TenantID != webroot.TenantID {
		response.WriteError(w, http.StatusForbidden, "hostname does not belong to this tenant")
		return
	}

	updated, err := h.hostingClient.UpdateFQDN(r.Context(), fqdnID, map[string]any{
		"webroot_id": webrootID,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to attach hostname")
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

// DetachFQDN removes a hostname from a webroot by clearing its webroot_id.
//
//	@Summary      Detach FQDN from webroot
//	@Description  Removes a hostname from a webroot by clearing its webroot_id
//	@Tags         Webroots
//	@Produce      json
//	@Param        id      path      string  true  "Webroot ID"
//	@Param        fqdnId  path      string  true  "FQDN ID"
//	@Success      200     {object}  hosting.FQDN
//	@Failure      400     {object}  response.ErrorResponse
//	@Failure      403     {object}  response.ErrorResponse
//	@Failure      404     {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/fqdns/{fqdnId}/detach [post]
func (h *Webroot) DetachFQDN(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	fqdnID, err := request.RequireID(chi.URLParam(r, "fqdnId"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	// Verify the FQDN belongs to this webroot
	fqdn, err := h.hostingClient.GetFQDN(r.Context(), fqdnID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "hostname not found")
		return
	}
	if fqdn.WebrootID == nil || *fqdn.WebrootID != webrootID {
		response.WriteError(w, http.StatusBadRequest, "hostname is not attached to this webroot")
		return
	}

	updated, err := h.hostingClient.UpdateFQDN(r.Context(), fqdnID, map[string]any{
		"webroot_id": nil,
	})
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to detach hostname")
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

// updateWebrootRequest is the request body for updating a webroot.
type updateWebrootRequest struct {
	Runtime        string `json:"runtime"`
	RuntimeVersion string `json:"runtime_version"`
	PublicFolder   string `json:"public_folder"`
}

// Update modifies a webroot's runtime, runtime_version, and public_folder.
//
//	@Summary      Update webroot
//	@Description  Modifies a webroot's runtime, runtime version, and public folder
//	@Tags         Webroots
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string                true  "Webroot ID"
//	@Param        body  body      updateWebrootRequest  true  "Webroot settings"
//	@Success      200   {object}  hosting.Webroot
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id} [put]
func (h *Webroot) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	var body updateWebrootRequest
	if err := request.Decode(r, &body); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	result, err := h.hostingClient.UpdateWebroot(r.Context(), id, body)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update webroot")
		return
	}

	response.WriteJSON(w, http.StatusOK, result)
}

// ListRuntimes returns the available runtimes for a webroot's cluster, grouped by runtime type.
//
//	@Summary      List available runtimes
//	@Description  Returns available runtimes for a webroot's cluster, grouped by runtime type
//	@Tags         Webroots
//	@Produce      json
//	@Param        id   path      string  true  "Webroot ID"
//	@Success      200  {object}  map[string][]hosting.RuntimeGroup
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/runtimes [get]
func (h *Webroot) ListRuntimes(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	tenant, err := h.hostingClient.GetTenant(r.Context(), webroot.TenantID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch tenant")
		return
	}

	flat, err := h.hostingClient.ListClusterRuntimes(r.Context(), tenant.ClusterID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch runtimes")
		return
	}

	// Group flat ClusterRuntime items into RuntimeGroup (runtime → versions).
	groups := groupRuntimes(flat)

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": groups})
}

// groupRuntimes converts flat ClusterRuntime items into grouped RuntimeGroups.
func groupRuntimes(flat []hosting.ClusterRuntime) []hosting.RuntimeGroup {
	order := make([]string, 0)
	m := make(map[string][]string)
	for _, r := range flat {
		if !r.Available {
			continue
		}
		if _, ok := m[r.Runtime]; !ok {
			order = append(order, r.Runtime)
		}
		m[r.Runtime] = append(m[r.Runtime], r.Version)
	}
	groups := make([]hosting.RuntimeGroup, 0, len(order))
	for _, rt := range order {
		groups = append(groups, hosting.RuntimeGroup{Runtime: rt, Versions: m[rt]})
	}
	return groups
}
