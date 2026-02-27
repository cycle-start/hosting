package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/model"
	"github.com/go-chi/chi/v5"
)

type Dashboard struct {
	customerSvc     *core.CustomerService
	subscriptionSvc *core.SubscriptionService
	moduleSvc       *core.ModuleService
}

func NewDashboard(customerSvc *core.CustomerService, subscriptionSvc *core.SubscriptionService, moduleSvc *core.ModuleService) *Dashboard {
	return &Dashboard{
		customerSvc:     customerSvc,
		subscriptionSvc: subscriptionSvc,
		moduleSvc:       moduleSvc,
	}
}

type dashboardResponse struct {
	CustomerName   string               `json:"customer_name"`
	Subscriptions  []model.Subscription `json:"subscriptions"`
	EnabledModules []string             `json:"enabled_modules"`
}

// Get returns the dashboard data for a customer.
//
//	@Summary      Get customer dashboard
//	@Description  Returns subscriptions and enabled modules for a customer
//	@Tags         Dashboard
//	@Produce      json
//	@Param        id   path      string  true  "Customer ID"
//	@Success      200  {object}  dashboardResponse
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      401  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers/{id}/dashboard [get]
func (h *Dashboard) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	customerID, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check user has access to this customer
	hasAccess, err := h.customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}
	if !hasAccess {
		response.WriteError(w, http.StatusForbidden, "no access to this customer")
		return
	}

	customer, err := h.customerSvc.GetByID(r.Context(), customerID)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	subscriptions, err := h.subscriptionSvc.ListByCustomer(r.Context(), customerID)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}
	if subscriptions == nil {
		subscriptions = []model.Subscription{}
	}

	partner := middleware.GetPartner(r.Context())
	if partner == nil {
		response.WriteError(w, http.StatusBadRequest, "unable to resolve partner")
		return
	}

	enabledModules, err := h.moduleSvc.GetEnabledModules(r.Context(), partner.BrandID)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, dashboardResponse{
		CustomerName:   customer.Name,
		Subscriptions:  subscriptions,
		EnabledModules: enabledModules,
	})
}
