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

// Create registers an OIDC client.
func (h *OIDCClient) Create(w http.ResponseWriter, r *http.Request) {
	var req createOIDCClientRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.CreateClient(r.Context(), req.ID, req.Secret, req.Name, req.RedirectURIs); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]string{"id": req.ID})
}
