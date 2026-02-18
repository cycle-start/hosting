package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
)

type OIDCClient struct {
	svc *core.OIDCService
}

func NewOIDCClient(svc *core.OIDCService) *OIDCClient {
	return &OIDCClient{svc: svc}
}

type createOIDCClientRequest struct {
	ID           string   `json:"id" validate:"required"`
	Secret       string   `json:"secret" validate:"required"`
	Name         string   `json:"name" validate:"required"`
	RedirectURIs []string `json:"redirect_uris" validate:"required"`
}

// Create godoc
//
//	@Summary		Register an OIDC client
//	@Description	Registers a new OIDC client (relying party) with the given ID and secret. The client can then initiate OIDC authorization code flows to authenticate tenants. The client secret is stored as a bcrypt hash and cannot be retrieved after creation.
//	@Tags			OIDC
//	@Security		ApiKeyAuth
//	@Param			body body createOIDCClientRequest true "OIDC client details"
//	@Success		201 {object} map[string]string
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/oidc/clients [post]
func (h *OIDCClient) Create(w http.ResponseWriter, r *http.Request) {
	var req createOIDCClientRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.CreateClient(r.Context(), req.ID, req.Secret, req.Name, req.RedirectURIs); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]string{"id": req.ID})
}
