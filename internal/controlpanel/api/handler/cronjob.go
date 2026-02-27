package handler

import (
	"net/http"

	"github.com/edvin/hosting/internal/controlpanel/api/request"
	"github.com/edvin/hosting/internal/controlpanel/api/response"
	"github.com/edvin/hosting/internal/controlpanel/core"
	"github.com/edvin/hosting/internal/controlpanel/hosting"
	"github.com/go-chi/chi/v5"
)

type CronJobHandler struct {
	customerSvc   *core.CustomerService
	hostingClient *hosting.Client
}

func NewCronJobHandler(customerSvc *core.CustomerService, hostingClient *hosting.Client) *CronJobHandler {
	return &CronJobHandler{
		customerSvc:   customerSvc,
		hostingClient: hostingClient,
	}
}

// List returns all cron jobs for a webroot.
//
//	@Summary      List cron jobs
//	@Description  Returns all cron jobs for a webroot
//	@Tags         Cron Jobs
//	@Produce      json
//	@Param        id   path      string  true  "Webroot ID"
//	@Success      200  {object}  map[string][]hosting.CronJob
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/cron-jobs [get]
func (h *CronJobHandler) List(w http.ResponseWriter, r *http.Request) {
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

	jobs, err := h.hostingClient.ListCronJobsByWebroot(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to fetch cron jobs")
		return
	}
	if jobs == nil {
		jobs = []hosting.CronJob{}
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"items": jobs})
}

// Create creates a cron job for a webroot.
//
//	@Summary      Create cron job
//	@Description  Creates a new cron job for a webroot
//	@Tags         Cron Jobs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Webroot ID"
//	@Param        body  body      object  true  "Cron job configuration"
//	@Success      201   {object}  hosting.CronJob
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /webroots/{id}/cron-jobs [post]
func (h *CronJobHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	job, err := h.hostingClient.CreateCronJob(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to create cron job")
		return
	}

	response.WriteJSON(w, http.StatusCreated, job)
}

// Update updates a cron job.
//
//	@Summary      Update cron job
//	@Description  Updates a cron job's configuration
//	@Tags         Cron Jobs
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string  true  "Cron Job ID"
//	@Param        body  body      object  true  "Cron job configuration"
//	@Success      200   {object}  hosting.CronJob
//	@Failure      400   {object}  response.ErrorResponse
//	@Failure      403   {object}  response.ErrorResponse
//	@Failure      404   {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /cron-jobs/{id} [put]
func (h *CronJobHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.hostingClient.GetCronJob(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "cron job not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), job.WebrootID)
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

	updated, err := h.hostingClient.UpdateCronJob(r.Context(), id, req)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to update cron job")
		return
	}

	response.WriteJSON(w, http.StatusOK, updated)
}

// Enable enables a cron job.
//
//	@Summary      Enable cron job
//	@Description  Enables a cron job to start running on schedule
//	@Tags         Cron Jobs
//	@Param        id  path  string  true  "Cron Job ID"
//	@Success      204
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /cron-jobs/{id}/enable [post]
func (h *CronJobHandler) Enable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.hostingClient.GetCronJob(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "cron job not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), job.WebrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	if err := h.hostingClient.EnableCronJob(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to enable cron job")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Disable disables a cron job.
//
//	@Summary      Disable cron job
//	@Description  Disables a cron job to stop it from running
//	@Tags         Cron Jobs
//	@Param        id  path  string  true  "Cron Job ID"
//	@Success      204
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /cron-jobs/{id}/disable [post]
func (h *CronJobHandler) Disable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.hostingClient.GetCronJob(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "cron job not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), job.WebrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	if err := h.hostingClient.DisableCronJob(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to disable cron job")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Delete deletes a cron job.
//
//	@Summary      Delete cron job
//	@Description  Permanently removes a cron job
//	@Tags         Cron Jobs
//	@Param        id  path  string  true  "Cron Job ID"
//	@Success      204
//	@Failure      400  {object}  response.ErrorResponse
//	@Failure      403  {object}  response.ErrorResponse
//	@Failure      404  {object}  response.ErrorResponse
//	@Security     BearerAuth
//	@Router       /cron-jobs/{id} [delete]
func (h *CronJobHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	job, err := h.hostingClient.GetCronJob(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "cron job not found")
		return
	}

	webroot, err := h.hostingClient.GetWebroot(r.Context(), job.WebrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}

	if !authorizeResourceByTenant(w, r, h.customerSvc, webroot.TenantID) {
		return
	}

	if err := h.hostingClient.DeleteCronJob(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to delete cron job")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
