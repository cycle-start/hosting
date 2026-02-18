package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Daemon struct {
	svc      *core.DaemonService
	services *core.Services
}

func NewDaemon(services *core.Services) *Daemon {
	return &Daemon{svc: services.Daemon, services: services}
}

func (h *Daemon) ListByWebroot(w http.ResponseWriter, r *http.Request) {
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

	daemons, hasMore, err := h.svc.ListByWebroot(r.Context(), webrootID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(daemons) > 0 {
		nextCursor = daemons[len(daemons)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, daemons, nextCursor, hasMore)
}

func (h *Daemon) Create(w http.ResponseWriter, r *http.Request) {
	webrootID, err := request.RequireID(chi.URLParam(r, "webrootID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateDaemon
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
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

	// Validate proxy_path
	if req.ProxyPath != nil {
		pp := *req.ProxyPath
		if !strings.HasPrefix(pp, "/") {
			response.WriteError(w, http.StatusBadRequest, "proxy_path must start with /")
			return
		}
		if strings.Contains(pp, "..") || strings.ContainsRune(pp, 0) {
			response.WriteError(w, http.StatusBadRequest, "invalid proxy_path")
			return
		}
	}

	// Apply defaults
	numProcs := req.NumProcs
	if numProcs == 0 {
		numProcs = 1
	}
	stopSignal := req.StopSignal
	if stopSignal == "" {
		stopSignal = "TERM"
	}
	stopWaitSecs := req.StopWaitSecs
	if stopWaitSecs == 0 {
		stopWaitSecs = 30
	}
	maxMemoryMB := req.MaxMemoryMB
	if maxMemoryMB == 0 {
		maxMemoryMB = 512
	}
	env := req.Environment
	if env == nil {
		env = make(map[string]string)
	}

	// Resolve tenant name for port computation
	tenant, err := h.services.Tenant.GetByID(r.Context(), webroot.TenantID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, "failed to resolve tenant")
		return
	}

	daemonName := platform.NewName("daemon_")

	// Compute proxy port if proxy_path is set
	var proxyPort *int
	if req.ProxyPath != nil && *req.ProxyPath != "" {
		port := core.ComputeDaemonPort(tenant.Name, webroot.Name, daemonName)
		proxyPort = &port
	}

	now := time.Now()
	daemon := &model.Daemon{
		ID:           platform.NewID(),
		TenantID:     webroot.TenantID,
		WebrootID:    webrootID,
		Name:         daemonName,
		Command:      req.Command,
		ProxyPath:    req.ProxyPath,
		ProxyPort:    proxyPort,
		NumProcs:     numProcs,
		StopSignal:   stopSignal,
		StopWaitSecs: stopWaitSecs,
		MaxMemoryMB:  maxMemoryMB,
		Environment:  env,
		Enabled:      true,
		Status:       model.StatusPending,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.svc.Create(r.Context(), daemon); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, daemon)
}

func (h *Daemon) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, daemon.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, daemon)
}

func (h *Daemon) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateDaemon
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, daemon.TenantID) {
		return
	}

	if req.Command != nil {
		daemon.Command = *req.Command
	}
	if req.ProxyPath != nil {
		if *req.ProxyPath == "" {
			// Clear proxy_path and proxy_port
			daemon.ProxyPath = nil
			daemon.ProxyPort = nil
		} else {
			if !strings.HasPrefix(*req.ProxyPath, "/") {
				response.WriteError(w, http.StatusBadRequest, "proxy_path must start with /")
				return
			}
			daemon.ProxyPath = req.ProxyPath
			// Recompute port
			tenant, err := h.services.Tenant.GetByID(r.Context(), daemon.TenantID)
			if err != nil {
				response.WriteError(w, http.StatusInternalServerError, "failed to resolve tenant")
				return
			}
			webroot, err := h.services.Webroot.GetByID(r.Context(), daemon.WebrootID)
			if err != nil {
				response.WriteError(w, http.StatusInternalServerError, "failed to resolve webroot")
				return
			}
			port := core.ComputeDaemonPort(tenant.Name, webroot.Name, daemon.Name)
			daemon.ProxyPort = &port
		}
	}
	if req.NumProcs != nil {
		daemon.NumProcs = *req.NumProcs
	}
	if req.StopSignal != nil {
		daemon.StopSignal = *req.StopSignal
	}
	if req.StopWaitSecs != nil {
		daemon.StopWaitSecs = *req.StopWaitSecs
	}
	if req.MaxMemoryMB != nil {
		daemon.MaxMemoryMB = *req.MaxMemoryMB
	}
	if req.Environment != nil {
		daemon.Environment = req.Environment
	}

	if err := h.svc.Update(r.Context(), daemon); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusAccepted, daemon)
}

func (h *Daemon) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, daemon.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Daemon) Enable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, daemon.TenantID) {
		return
	}

	if err := h.svc.Enable(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Daemon) Disable(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, daemon.TenantID) {
		return
	}

	if err := h.svc.Disable(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (h *Daemon) Retry(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	daemon, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}
	if !checkTenantBrand(w, r, h.services.Tenant, daemon.TenantID) {
		return
	}

	if err := h.svc.Retry(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
