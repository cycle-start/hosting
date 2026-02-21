package handler

import (
	"net/http"
	"time"

	"github.com/edvin/hosting/internal/api/request"
	"github.com/edvin/hosting/internal/api/response"
	"github.com/edvin/hosting/internal/core"
	"github.com/edvin/hosting/internal/model"
	"github.com/go-chi/chi/v5"
)

type Subscription struct {
	svc      *core.SubscriptionService
	services *core.Services
}

func NewSubscription(services *core.Services) *Subscription {
	return &Subscription{svc: services.Subscription, services: services}
}

func (h *Subscription) ListByTenant(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, tenantID) {
		return
	}

	pg := request.ParsePagination(r)

	subs, hasMore, err := h.svc.ListByTenant(r.Context(), tenantID, pg.Limit, pg.Cursor)
	if err != nil {
		response.WriteServiceError(w, err)
		return
	}

	var nextCursor string
	if hasMore && len(subs) > 0 {
		nextCursor = subs[len(subs)-1].ID
	}
	response.WritePaginated(w, http.StatusOK, subs, nextCursor, hasMore)
}

func (h *Subscription) Get(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	sub, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, sub.TenantID) {
		return
	}

	response.WriteJSON(w, http.StatusOK, sub)
}

func (h *Subscription) Create(w http.ResponseWriter, r *http.Request) {
	tenantID, err := request.RequireID(chi.URLParam(r, "tenantID"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	var req request.CreateSubscription
	if err := request.Decode(r, &req); err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, tenantID) {
		return
	}

	now := time.Now()
	sub := &model.Subscription{
		ID:        req.ID,
		TenantID:  tenantID,
		Name:      req.Name,
		Status:    model.StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.svc.Create(r.Context(), sub); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	response.WriteJSON(w, http.StatusCreated, sub)
}

func (h *Subscription) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := request.RequireID(chi.URLParam(r, "id"))
	if err != nil {
		response.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}

	sub, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		response.WriteError(w, http.StatusNotFound, err.Error())
		return
	}

	if !checkTenantBrand(w, r, h.services.Tenant, sub.TenantID) {
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		response.WriteServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
