package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	_ "github.com/edvin/hosting/internal/controlpanel/model" // imported for swag
)

type Me struct {
	userSvc     *core.UserService
	customerSvc *core.CustomerService
}

func NewMe(userSvc *core.UserService, customerSvc *core.CustomerService) *Me {
	return &Me{userSvc: userSvc, customerSvc: customerSvc}
}

// Get returns the current authenticated user's profile.
//
//	@Summary      Get current user
//	@Description  Returns the profile of the currently authenticated user
//	@Tags         Profile
//	@Produce      json
//	@Success      200  {object}  model.User
//	@Failure      401  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /me [get]
func (h *Me) Get(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	user, err := h.userSvc.GetByID(r.Context(), claims.Sub)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, user)
}

type updateMeRequest struct {
	Locale         *string `json:"locale"`
	LastCustomerID *string `json:"last_customer_id"`
	DisplayName    *string `json:"display_name"`
}

// Update modifies the current user's profile fields.
//
//	@Summary      Update current user
//	@Description  Update locale, display name, or last selected customer for the current user
//	@Tags         Profile
//	@Accept       json
//	@Produce      json
//	@Param        body  body      updateMeRequest  true  "Fields to update"
//	@Success      200   {object}  model.User
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      401   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /me [patch]
func (h *Me) Update(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return
	}

	var req updateMeRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.Locale != nil {
		if err := h.userSvc.UpdateLocale(r.Context(), claims.Sub, *req.Locale); err != nil {
			response.WriteServiceError(w, err)
			return
		}
	}

	if req.DisplayName != nil {
		if err := h.userSvc.UpdateDisplayName(r.Context(), claims.Sub, req.DisplayName); err != nil {
			response.WriteServiceError(w, err)
			return
		}
	}

	if req.LastCustomerID != nil {
		// Verify the user has access to this customer
		hasAccess, err := h.customerSvc.UserHasAccess(r.Context(), claims.Sub, *req.LastCustomerID)
		if err != nil {
			response.WriteServiceError(w, err)
			return
		}
		if !hasAccess {
			response.WriteError(w, http.StatusForbidden, "no access to this customer")
			return
		}
		if err := h.userSvc.UpdateLastCustomer(r.Context(), claims.Sub, *req.LastCustomerID); err != nil {
			response.WriteServiceError(w, err)
			return
		}
	}

	// Return updated user
	user, err := h.userSvc.GetByID(r.Context(), claims.Sub)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, user)
}
