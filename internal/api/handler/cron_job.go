package handler

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

// cronScheduleRegex validates a basic 5-field cron expression.
var cronScheduleRegex = regexp.MustCompile(`^(\S+\s+){4}\S+$`)

type CronJob struct {
	svc      *core.CronJobService
	services *core.Services
}

func NewCronJob(services *core.Services) *CronJob {
	return &CronJob{svc: services.CronJob, services: services}
}

// ListByWebroot godoc
//
//	@Summary		List cron jobs for a webroot
//	@Description	Returns a paginated list of cron jobs belonging to the specified webroot.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			webrootID path string true "Webroot ID"
//	@Param			limit query int false "Page size" default(50)
//	@Param			cursor query string false "Pagination cursor"
//	@Success		200 {object} response.PaginatedResponse{items=[]model.CronJob}
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/webroots/{webrootID}/cron-jobs [get]
func (h *CronJob) ListByWebroot(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
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

	pg := request.ParsePagination(r)

	cronJobs, hasMore, err := h.svc.ListByWebroot(r.Context(), webrootID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(cronJobs) > 0 {
		nextCursor = cronJobs[len(cronJobs)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, cronJobs, nextCursor, hasMore)
}

// Create godoc
//
//	@Summary		Create a cron job
//	@Description	Creates a scheduled cron job for a webroot. Async — returns 202 and triggers a Temporal workflow to configure the cron job on the web server.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			webrootID path string true "Webroot ID"
//	@Param			body body request.CreateCronJob true "Cron job details"
//	@Success		202 {object} model.CronJob
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/webroots/{webrootID}/cron-jobs [post]
func (h *CronJob) Create(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateCronJob
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Validate cron schedule (5 fields separated by spaces)
	if !cronScheduleRegex.MatchString(req.Schedule) {
		response.WriteError(w, http.StatusBadRequest, "invalid cron schedule: must be 5 fields (minute hour day month weekday)")
		return
	}

	// Validate working_directory
	if strings.Contains(req.WorkingDirectory, "..") || strings.HasPrefix(req.WorkingDirectory, "/") || strings.ContainsRune(req.WorkingDirectory, 0) {
		response.WriteError(w, http.StatusBadRequest, "invalid working_directory")
		return
	}

	webroot, err := h.services.Webroot.GetByID(r.Context(), webrootID)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, "webroot not found")
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, webroot.TenantID) {
		return
	}
	if webroot.Status != model.StatusActive {
		response.WriteError(w, http.StatusBadRequest, "webroot is not active")
		return
	}

	// Apply defaults
	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds == 0 {
		timeoutSeconds = 3600
	}
	maxMemoryMB := req.MaxMemoryMB
	if maxMemoryMB == 0 {
		maxMemoryMB = 512
	}

	now := time.Now()
	cronJob := &model.CronJob{
		ID:               platform.NewID(),
		TenantID:         webroot.TenantID,
		WebrootID:        webrootID,
		Name:             platform.NewName("cron_"),
		Schedule:         req.Schedule,
		Command:          req.Command,
		WorkingDirectory: req.WorkingDirectory,
		Enabled:          false,
		TimeoutSeconds:   timeoutSeconds,
		MaxMemoryMB:      maxMemoryMB,
		Status:           model.StatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := h.svc.Create(r.Context(), cronJob); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, cronJob)
}

// Get godoc
//
//	@Summary		Get a cron job
//	@Description	Returns a single cron job by ID.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			id path string true "Cron Job ID"
//	@Success		200 {object} model.CronJob
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Router			/cron-jobs/{id} [get]
func (h *CronJob) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cronJob, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, cronJob.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, cronJob)
}

// Update godoc
//
//	@Summary		Update a cron job
//	@Description	Partial update of a cron job — supports changing schedule, command, working directory, timeout, and memory limit. Async — returns 202 and triggers re-convergence.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			id path string true "Cron Job ID"
//	@Param			body body request.UpdateCronJob true "Cron job updates"
//	@Success		202 {object} model.CronJob
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		404 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/cron-jobs/{id} [put]
func (h *CronJob) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateCronJob
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cronJob, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, cronJob.TenantID) {
		return
	}

	if req.Schedule != nil {
		if !cronScheduleRegex.MatchString(*req.Schedule) {
			response.WriteError(w, http.StatusBadRequest, "invalid cron schedule: must be 5 fields (minute hour day month weekday)")
			return
		}
		cronJob.Schedule = *req.Schedule
	}
	if req.Command != nil {
		cronJob.Command = *req.Command
	}
	if req.WorkingDirectory != nil {
		if strings.Contains(*req.WorkingDirectory, "..") || strings.HasPrefix(*req.WorkingDirectory, "/") || strings.ContainsRune(*req.WorkingDirectory, 0) {
			response.WriteError(w, http.StatusBadRequest, "invalid working_directory")
			return
		}
		cronJob.WorkingDirectory = *req.WorkingDirectory
	}
	if req.TimeoutSeconds != nil {
		cronJob.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.MaxMemoryMB != nil {
		cronJob.MaxMemoryMB = *req.MaxMemoryMB
	}

	if err := h.svc.Update(r.Context(), cronJob); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, cronJob)
}

// Delete godoc
//
//	@Summary		Delete a cron job
//	@Description	Deletes a cron job. Async — returns 202 and triggers a Temporal workflow.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			id path string true "Cron Job ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/cron-jobs/{id} [delete]
func (h *CronJob) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cronJob, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, cronJob.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Enable godoc
//
//	@Summary		Enable a cron job
//	@Description	Enables a cron job that is currently in active state. Async — returns 202 and triggers a Temporal workflow.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			id path string true "Cron Job ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/cron-jobs/{id}/enable [post]
func (h *CronJob) Enable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cronJob, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, cronJob.TenantID) {
		return
	}

	if err := h.svc.Enable(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Disable godoc
//
//	@Summary		Disable a cron job
//	@Description	Disables a cron job that is currently in active state. Async — returns 202 and triggers a Temporal workflow.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			id path string true "Cron Job ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/cron-jobs/{id}/disable [post]
func (h *CronJob) Disable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cronJob, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, cronJob.TenantID) {
		return
	}

	if err := h.svc.Disable(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

// Retry godoc
//
//	@Summary		Retry a failed cron job
//	@Description	Re-triggers the provisioning workflow for a cron job in failed state. Async — returns 202 and starts a new Temporal workflow.
//	@Tags			Cron Jobs
//	@Security		ApiKeyAuth
//	@Param			id path string true "Cron Job ID"
//	@Success		202
//	@Failure		400 {object} response.ErrorResponse
//	@Failure		500 {object} response.ErrorResponse
//	@Router			/cron-jobs/{id}/retry [post]
func (h *CronJob) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	cronJob, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, cronJob.TenantID) {
		return
	}

	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
