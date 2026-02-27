package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/middleware"
	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type EnvVar struct {
	customerSvc   *core.CustomerService
	hostingClient *hosting.Client
}

func NewEnvVar(customerSvc *core.CustomerService, hostingClient *hosting.Client) *EnvVar {
	return &EnvVar{
		customerSvc:   customerSvc,
		hostingClient: hostingClient,
	}
}

// authorizeWebroot verifies the caller has access to the webroot via tenant->customer mapping.
// Returns the webroot ID or writes an error response and returns empty string.
func (h *EnvVar) authorizeWebroot(w http.ResponseWriter, r *http.Request) string {
	claims := middleware.GetClaims(r.Context())
	if claims == nil {
		response.WriteError(w, http.StatusUnauthorized, "missing claims")
		return ""
	}

	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return ""
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return ""
	}

	customerID, err := h.customerSvc.GetCustomerIDByTenant(r.Context(), webroot.TenantID)
	if err != nil {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return ""
	}

	hasAccess, err := h.customerSvc.UserHasAccess(r.Context(), claims.Sub, customerID)
	if err != nil || !hasAccess {
		response.WriteError(w, http.StatusForbidden, "no access to this resource")
		return ""
	}

	return id
}

// setEnvVarsRequest is the request body for setting environment variables.
type setEnvVarsRequest struct {
	Vars []hosting.SetEnvVarEntry `json:"vars" validate:"required"`
}

// vaultEncryptRequest is the request body for vault encryption.
type vaultEncryptRequest struct {
	Plaintext string `json:"plaintext" validate:"required"`
}

// vaultDecryptRequest is the request body for vault decryption.
type vaultDecryptRequest struct {
	Token string `json:"token" validate:"required"`
}

// List returns all env vars for a webroot.
//
//	@Summary      List environment variables
//	@Description  Returns all environment variables for a webroot
//	@Tags         Environment Variables
//	@Produce      json
//	@Param        id   path      string  true  "Webroot ID"
//	@Success      200  {object}  map[string][]hosting.EnvVar
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      401  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/env-vars [get]
func (h *EnvVar) List(w http.ResponseWriter, r *http.Request) {
	webrootID := h.authorizeWebroot(w, r)
	if webrootID == "" {
		return
	}

	vars, err := h.hostingClient.ListEnvVars(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch env vars")
		return
	}
	if vars == nil {
		vars = []hosting.EnvVar{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": vars})
}

// Set replaces all env vars for a webroot (bulk PUT).
//
//	@Summary      Set environment variables
//	@Description  Replaces all environment variables for a webroot
//	@Tags         Environment Variables
//	@Accept       json
//	@Produce      json
//	@Param        id    path  string              true  "Webroot ID"
//	@Param        body  body  setEnvVarsRequest   true  "Environment variables"
//	@Success      202
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      401  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/env-vars [put]
func (h *EnvVar) Set(w http.ResponseWriter, r *http.Request) {
	webrootID := h.authorizeWebroot(w, r)
	if webrootID == "" {
		return
	}

	var req setEnvVarsRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.hostingClient.SetEnvVars(r.Context(), webrootID, req.Vars); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to set env vars")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Delete removes a single env var by name.
//
//	@Summary      Delete environment variable
//	@Description  Removes a single environment variable by name
//	@Tags         Environment Variables
//	@Param        id    path  string  true  "Webroot ID"
//	@Param        name  path  string  true  "Environment variable name"
//	@Success      202
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      401  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/env-vars/{name} [delete]
func (h *EnvVar) Delete(w http.ResponseWriter, r *http.Request) {
	webrootID := h.authorizeWebroot(w, r)
	if webrootID == "" {
		return
	}

	name := chi.URLParam(r, "name")
	if name == "" {
		response.WriteError(w, http.StatusBadRequest, "missing env var name")
		return
	}

	if err := h.hostingClient.DeleteEnvVar(r.Context(), webrootID, name); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete env var")
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// VaultEncrypt encrypts a plaintext value and returns a vault token.
//
//	@Summary      Encrypt value with vault
//	@Description  Encrypts a plaintext value and returns a vault token for use in env vars
//	@Tags         Environment Variables
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string               true  "Webroot ID"
//	@Param        body  body      vaultEncryptRequest   true  "Value to encrypt"
//	@Success      200   {object}  map[string]string
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      401   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/vault/encrypt [post]
func (h *EnvVar) VaultEncrypt(w http.ResponseWriter, r *http.Request) {
	webrootID := h.authorizeWebroot(w, r)
	if webrootID == "" {
		return
	}

	var req vaultEncryptRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := h.hostingClient.VaultEncrypt(r.Context(), webrootID, req.Plaintext)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"token": token})
}

// VaultDecrypt decrypts a vault token and returns the plaintext.
//
//	@Summary      Decrypt vault token
//	@Description  Decrypts a vault token and returns the plaintext value
//	@Tags         Environment Variables
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string               true  "Webroot ID"
//	@Param        body  body      vaultDecryptRequest   true  "Token to decrypt"
//	@Success      200   {object}  map[string]string
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      401   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/vault/decrypt [post]
func (h *EnvVar) VaultDecrypt(w http.ResponseWriter, r *http.Request) {
	webrootID := h.authorizeWebroot(w, r)
	if webrootID == "" {
		return
	}

	var req vaultDecryptRequest
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	plaintext, err := h.hostingClient.VaultDecrypt(r.Context(), webrootID, req.Token)
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, "decryption failed")
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]string{"plaintext": plaintext})
}
