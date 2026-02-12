package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/edvin/hosting/internal/platform"
	"github.com/go-chi/chi/v5"
)

type Webroot struct {
	svc *core.WebrootService
}

func NewWebroot(svc *core.WebrootService) *Webroot {
	return &Webroot{svc: svc}
}

func (h *Webroot) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	pg := request.ParsePagination(r)

	webroots, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var nextCursor string
	if hasMore && len(webroots) > 0 {
		nextCursor = webroots[len(webroots)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, webroots, nextCursor, hasMore)
}

func (h *Webroot) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateWebroot
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now()
	runtimeConfig := req.RuntimeConfig
	if runtimeConfig == nil {
		runtimeConfig = json.RawMessage(`{}`)
	}
	webroot := &model.Webroot{
		ID:             platform.NewShortID(),
		TenantID:       tenantID,
		Name:           req.Name,
		Runtime:        req.Runtime,
		RuntimeVersion: req.RuntimeVersion,
		RuntimeConfig:  runtimeConfig,
		PublicFolder:   req.PublicFolder,
		Status:         model.StatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := h.svc.Create(r.Context(), webroot); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, webroot)
}

func (h *Webroot) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusOK, webroot)
}

func (h *Webroot) Update(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.UpdateWebroot
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	webroot, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if req.Runtime != "" {
		webroot.Runtime = req.Runtime
	}
	if req.RuntimeVersion != "" {
		webroot.RuntimeVersion = req.RuntimeVersion
	}
	if req.RuntimeConfig != nil {
		webroot.RuntimeConfig = req.RuntimeConfig
	}
	if req.PublicFolder != nil {
		webroot.PublicFolder = *req.PublicFolder
	}

	if err := h.svc.Update(r.Context(), webroot); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	response.WriteJSON(w, http.StatusAccepted, webroot)
}

func (h *Webroot) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
