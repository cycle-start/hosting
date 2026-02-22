package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/go-chi/chi/v5"
)

type Vault struct {
	svc      *core.WebrootEnvVarService
	services *core.Services
}

func NewVault(services *core.Services) *Vault {
	return &Vault{svc: services.WebrootEnvVar, services: services}
}

// Encrypt encrypts a plaintext value and returns a vault token.
func (h *Vault) Encrypt(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.VaultEncrypt
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.services.Webroot.GetByID(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	token, err := h.svc.VaultEncrypt(r.Context(), webrootID, req.Plaintext)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

// Decrypt decrypts a vault token and returns the plaintext.
func (h *Vault) Decrypt(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.VaultDecrypt
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.services.Webroot.GetByID(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}

	plaintext, err := h.svc.VaultDecrypt(r.Context(), webrootID, req.Token)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "invalid vault token: decryption failed")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"plaintext": plaintext})
}
