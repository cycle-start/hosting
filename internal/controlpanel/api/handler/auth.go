package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
)

type Auth struct {
	svc *core.AuthService
}

func NewAuth(svc *core.AuthService) *Auth {
	return &Auth{svc: svc}
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login authenticates a user and returns a JWT token.
//
//	@Summary      Authenticate user
//	@Description  Authenticate with email and password to receive a JWT token
//	@Tags         Authentication
//	@Accept       json
//	@Produce      json
//	@Param        body  body      loginRequest  true  "Login credentials"
//	@Success      200   {object}  loginResponse
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      401   {object}  response.ErrorResponse
//	@Router       /auth/login [post]
func (h *Auth) Login(w http.ResponseWriter, r *http.Request) {
	partner := middleware.GetPartner(r.Context())
	if partner == nil {
		response.WriteError(w, http.StatusBadRequest, "unable to resolve partner")
		return
	}

	var req loginRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := h.svc.Login(r.Context(), req.Email, req.Password, partner.ID)
	if err != nil {
		response.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, loginResponse{Token: token})
}
