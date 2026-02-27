package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type DaemonHandler struct {
	customerSvc   *core.CustomerService
	hostingClient *hosting.Client
}

func NewDaemonHandler(customerSvc *core.CustomerService, hostingClient *hosting.Client) *DaemonHandler {
	return &DaemonHandler{
		customerSvc:   customerSvc,
		hostingClient: hostingClient,
	}
}

// List returns all daemons for a webroot.
//
//	@Summary      List daemons
//	@Description  Returns all daemons for a webroot
//	@Tags         Daemons
//	@Produce      json
//	@Param        id   path      string  true  "Webroot ID"
//	@Success      200  {object}  map[string][]hosting.Daemon
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/daemons [get]
func (h *DaemonHandler) List(w http.ResponseWriter, r *http.Request) {
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

	daemons, err := h.hostingClient.ListDaemonsByWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch daemons")
		return
	}
	if daemons == nil {
		daemons = []hosting.Daemon{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": daemons})
}

// Create creates a daemon for a webroot.
//
//	@Summary      Create daemon
//	@Description  Creates a new daemon for a webroot
//	@Tags         Daemons
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Webroot ID"
//	@Param        body  body      object  true  "Daemon configuration"
//	@Success      201   {object}  hosting.Daemon
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/daemons [post]
func (h *DaemonHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.hostingClient.CreateDaemon(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create daemon")
		return
	}

	response.WriteJSON(w, http.StatusCreated, daemon)
}

// Update updates a daemon.
//
//	@Summary      Update daemon
//	@Description  Updates a daemon's configuration
//	@Tags         Daemons
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Daemon ID"
//	@Param        body  body      object  true  "Daemon configuration"
//	@Success      200   {object}  hosting.Daemon
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /daemons/{id} [put]
func (h *DaemonHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.hostingClient.GetDaemon(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "daemon not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), daemon.WebrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	var req any
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	updated, err := h.hostingClient.UpdateDaemon(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update daemon")
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

// Enable enables a daemon.
//
//	@Summary      Enable daemon
//	@Description  Enables a daemon to start running
//	@Tags         Daemons
//	@Param        id  path  string  true  "Daemon ID"
//	@Success      204
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /daemons/{id}/enable [post]
func (h *DaemonHandler) Enable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.hostingClient.GetDaemon(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "daemon not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), daemon.WebrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	if err := h.hostingClient.EnableDaemon(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to enable daemon")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Disable disables a daemon.
//
//	@Summary      Disable daemon
//	@Description  Disables a daemon to stop it from running
//	@Tags         Daemons
//	@Param        id  path  string  true  "Daemon ID"
//	@Success      204
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /daemons/{id}/disable [post]
func (h *DaemonHandler) Disable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.hostingClient.GetDaemon(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "daemon not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), daemon.WebrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	if err := h.hostingClient.DisableDaemon(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to disable daemon")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Delete deletes a daemon.
//
//	@Summary      Delete daemon
//	@Description  Permanently removes a daemon
//	@Tags         Daemons
//	@Param        id  path  string  true  "Daemon ID"
//	@Success      204
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /daemons/{id} [delete]
func (h *DaemonHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.hostingClient.GetDaemon(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "daemon not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), daemon.WebrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteDaemon(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete daemon")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
