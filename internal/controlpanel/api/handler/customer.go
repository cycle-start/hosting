package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/model"
)

type Customer struct {
	svc *core.CustomerService
}

func NewCustomer(svc *core.CustomerService) *Customer {
	return &Customer{svc: svc}
}

// List returns all customers accessible to the current user.
//
//	@Summary      List customers
//	@Description  Returns all customers the authenticated user has access to
//	@Tags         Customers
//	@Produce      json
//	@Success      200  {object}  map[string][]model.Customer
//	@Failure      401  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /customers [get]
func (h *Customer) List(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	customers, err := h.svc.ListByUser(r.Context(), claims.Sub)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	if customers == nil {
		customers = []model.Customer{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": customers})
}
