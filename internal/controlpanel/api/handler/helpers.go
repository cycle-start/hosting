package handler

import (
	"net/http"
	"sync"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/go-chi/chi/v5"
)

// authorizeCustomer extracts claims, reads the customer ID from paramName, and
// checks that the caller has access. Returns the customer ID or writes an error.
func authorizeCustomer(w http.ResponseWriter, r *http.Request, customerSvc *core.CustomerService, paramName string) (string, bool) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return "", false
	}

	customerID, err := request.RequireID(chi.URLParam(r, paramName))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return "", false
	}

	hasAccess, err := customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil {
		response.WriteServiceError(w, err)
		return "", false
	}
	if !hasAccess {
		response.WriteError(w, http.StatusForbidden, "no access to this customer")
		return "", false
	}

	return customerID, true
}

// authorizeResourceByTenant maps a tenant ID to a customer and checks that
// the caller has access. Returns true if authorized.
func authorizeResourceByTenant(w http.ResponseWriter, r *http.Request, customerSvc *core.CustomerService, tenantID string) bool {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return false
	}

	customerID, err := customerSvc.GetCustomerIDByTenant(r.Context(), tenantID)
	if err != nil {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return false
	}

	hasAccess, err := customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil || !hasAccess {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return false
	}

	return true
}

// authorizeResourceBySubscription maps a subscription ID to a customer and checks
// that the caller has access. Returns true if authorized.
func authorizeResourceBySubscription(w http.ResponseWriter, r *http.Request, customerSvc *core.CustomerService, subscriptionID string) bool {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return false
	}

	customerID, err := customerSvc.GetCustomerIDBySubscription(r.Context(), subscriptionID)
	if err != nil {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return false
	}

	hasAccess, err := customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil || !hasAccess {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return false
	}

	return true
}

// listByCustomerWithModule implements the fan-out pattern: get subscriptions
// with a given module, collect tenant IDs, fan out concurrent fetches, merge.
func listByCustomerWithModule[T any](
	w http.ResponseWriter,
	r *http.Request,
	customerSvc *core.CustomerService,
	subscriptionSvc *core.SubscriptionService,
	module string,
	fetcher func(tenantID string) ([]T, error),
) {
	customerID, ok := authorizeCustomer(w, r, customerSvc, "cid")
	if !ok {
		return
	}

	subs, err := subscriptionSvc.ListByCustomerWithModule(r.Context(), customerID, module)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var tenantIDs []string
	for _, sub := range subs {
		tenantIDs = append(tenantIDs, sub.TenantID)
	}

	if len(tenantIDs) == 0 {
		response.WriteJSON(w, http.StatusOK, map[string]any{"items": []T{}})
		return
	}

	type result struct {
		items []T
		err   error
	}
	results := make([]result, len(tenantIDs))
	var wg sync.WaitGroup

	for i, tid := range tenantIDs {
		wg.Add(1)
		go func(idx int, tenantID string) {
			defer wg.Done()
			items, err := fetcher(tenantID)
			results[idx] = result{items: items, err: err}
		}(i, tid)
	}
	wg.Wait()

	var all []T
	for _, res := range results {
		if res.err == nil {
			all = append(all, res.items...)
		}
	}
	if all == nil {
		all = []T{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": all})
}
